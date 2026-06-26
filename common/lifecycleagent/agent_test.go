//  Copyright (c) 2026 Metaform Systems, Inc
//
//  This program and the accompanying materials are made available under the
//  terms of the Apache License, Version 2.0 which is available at
//  https://www.apache.org/licenses/LICENSE-2.0
//
//  SPDX-License-Identifier: Apache-2.0
//
//  Contributors:
//       Metaform Systems, Inc. - initial API and implementation
//

package lifecycleagent_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	agentTestTimeout = 30 * time.Second
	pollInterval     = 100 * time.Millisecond
	bucket           = "cfm-bucket"
	streamName       = "events-stream"
)

type testEvent struct {
	ID string `json:"id"`
}

// recordingProcessor records each received event and delegates the returned error to a configurable hook so individual
// tests can drive the ack/nak behaviour.
type recordingProcessor struct {
	calls    atomic.Int32
	received chan lifecycleagent.EventContext[testEvent]
	onCall   func(call int32) error
}

func (p *recordingProcessor) Process(_ context.Context, evt lifecycleagent.EventContext[testEvent]) error {
	call := p.calls.Add(1)
	select {
	case p.received <- evt:
	default:
	}
	if p.onCall != nil {
		return p.onCall(call)
	}
	return nil
}

func setupAgentEnv(t *testing.T, prefix, uri string) {
	// viper uppercases the env prefix and key, so the environment variables must be uppercase.
	p := strings.ToUpper(prefix)
	t.Setenv(p+"_URI", uri)
	t.Setenv(p+"_BUCKET", bucket)
	t.Setenv(p+"_STREAM", streamName)
}

// launchAgent starts an agent and waits for its durable consumer to exist so that, under InterestPolicy, messages
// published afterwards are retained. The agentName must be lowercase and free of separators so it equals the durable.
func launchAgent(t *testing.T, ctx context.Context, nt *natsfixtures.NatsTestContainer, agentName, subject string, proc lifecycleagent.EventProcessor[testEvent]) chan struct{} {
	cfg := lifecycleagent.LauncherConfig[testEvent]{
		AgentName:    agentName,
		ServiceName:  "cfm.agent." + agentName,
		ConfigPrefix: agentName,
		Subjects:     []string{subject},
		NewProcessor: func(*lifecycleagent.AgentContext) lifecycleagent.EventProcessor[testEvent] {
			return proc
		},
	}

	shutdown := make(chan struct{})
	go lifecycleagent.LaunchAgent(shutdown, cfg)

	require.Eventually(t, func() bool {
		stream, err := nt.Client.JetStream.Stream(ctx, streamName)
		if err != nil {
			return false
		}
		_, err = stream.Consumer(ctx, agentName)
		return err == nil
	}, agentTestTimeout, pollInterval, "agent durable consumer should be created")

	return shutdown
}

func TestLifecycleAgent_DispatchesDecodedEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, bucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subject := "events.test.created"
	setupAgentEnv(t, "dispatchagent", nt.URI)

	proc := &recordingProcessor{received: make(chan lifecycleagent.EventContext[testEvent], 8)}
	shutdown := launchAgent(t, ctx, nt, "dispatchagent", subject, proc)
	defer close(shutdown)

	payload, _ := json.Marshal(testEvent{ID: "evt-1"})
	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, subject, payload)
	require.NoError(t, err)

	select {
	case evt := <-proc.received:
		assert.Equal(t, "evt-1", evt.Payload.ID)
		assert.Equal(t, subject, evt.Subject)
		assert.JSONEq(t, string(payload), string(evt.Raw))
	case <-time.After(agentTestTimeout):
		t.Fatal("timed out waiting for the event to be dispatched")
	}
}

func TestLifecycleAgent_RecoverableErrorIsRedelivered(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, bucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subject := "events.retry.created"
	setupAgentEnv(t, "retryagent", nt.URI)

	proc := &recordingProcessor{
		received: make(chan lifecycleagent.EventContext[testEvent], 8),
		onCall: func(call int32) error {
			if call == 1 {
				return types.NewRecoverableError("transient failure")
			}
			return nil
		},
	}
	shutdown := launchAgent(t, ctx, nt, "retryagent", subject, proc)
	defer close(shutdown)

	payload, _ := json.Marshal(testEvent{ID: "evt-retry"})
	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, subject, payload)
	require.NoError(t, err)

	// A recoverable error NAKs the message, so it must be delivered more than once.
	assert.Eventually(t, func() bool {
		return proc.calls.Load() >= 2
	}, agentTestTimeout, pollInterval, "recoverable error should cause redelivery")
}

func TestLifecycleAgent_FatalErrorIsNotRedelivered(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, bucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subject := "events.fatal.created"
	setupAgentEnv(t, "fatalagent", nt.URI)

	proc := &recordingProcessor{
		received: make(chan lifecycleagent.EventContext[testEvent], 8),
		onCall: func(int32) error {
			return assert.AnError // non-recoverable
		},
	}
	shutdown := launchAgent(t, ctx, nt, "fatalagent", subject, proc)
	defer close(shutdown)

	payload, _ := json.Marshal(testEvent{ID: "evt-fatal"})
	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, subject, payload)
	require.NoError(t, err)

	// Wait until the message has been processed once.
	assert.Eventually(t, func() bool {
		return proc.calls.Load() >= 1
	}, agentTestTimeout, pollInterval, "message should be processed once")

	// A fatal error ACKs (drops) the message, so it must not be redelivered.
	time.Sleep(2 * time.Second)
	assert.Equal(t, int32(1), proc.calls.Load(), "fatal error should not cause redelivery")
}

func TestLifecycleAgent_ShutdownStopsProcessing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, bucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subject := "events.shutdown.created"
	setupAgentEnv(t, "shutdownagent", nt.URI)

	proc := &recordingProcessor{received: make(chan lifecycleagent.EventContext[testEvent], 8)}
	shutdown := launchAgent(t, ctx, nt, "shutdownagent", subject, proc)

	// Stop the agent, then publish — the (now-stopped) consumer should not process the message.
	close(shutdown)
	time.Sleep(time.Second)

	payload, _ := json.Marshal(testEvent{ID: "evt-after-shutdown"})
	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, subject, payload)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
	assert.Equal(t, int32(0), proc.calls.Load(), "no messages should be processed after shutdown")
}
