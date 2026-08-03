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

package e2etests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/fixtures"
	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	eventsStream = "events-stream"
	eventsPoll   = 100 * time.Millisecond
)

// itemCreated is a domain payload carried in the CloudEvent Data field.
type itemCreated struct {
	ItemID string `json:"itemId"`
	Tenant string `json:"tenant"`
}

// captureProcessor records the first decoded CloudEvent it receives.
type captureProcessor struct {
	received chan lifecycleagent.EventContext[lifecycleagent.CloudEvent[itemCreated]]
}

func (p *captureProcessor) Process(_ context.Context, evt lifecycleagent.EventContext[lifecycleagent.CloudEvent[itemCreated]]) error {
	select {
	case p.received <- evt:
	default:
	}
	return nil
}

// Test_VerifyLifecycleAgentDispatchesCloudEvent verifies the lifecycle agent framework end-to-end: a CloudEvents
// envelope published to the shared event stream is delivered to a standalone agent, decoded into its typed payload, and
// dispatched to the agent's EventProcessor with the originating subject and raw body intact.
func Test_VerifyLifecycleAgentDispatchesCloudEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, cfmBucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subject := "events.item.created"
	agentName := "lifecycleagent"

	// The shared event stream is provisioned out-of-band, as in production; the agent only binds a consumer to it.
	_, err = nt.Client.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      eventsStream,
		Retention: jetstream.InterestPolicy,
		Storage:   jetstream.MemoryStorage,
		Subjects:  []string{subject},
	})
	require.NoError(t, err)

	fixtures.IsolateConfig(t)

	// viper uppercases the env prefix and key, so the environment variables must be uppercase.
	p := strings.ToUpper(agentName)
	t.Setenv(p+"_URI", nt.URI)
	t.Setenv(p+"_BUCKET", cfmBucket)
	t.Setenv(p+"_STREAM", eventsStream)

	proc := &captureProcessor{received: make(chan lifecycleagent.EventContext[lifecycleagent.CloudEvent[itemCreated]], 1)}
	cfg := lifecycleagent.LauncherConfig[lifecycleagent.CloudEvent[itemCreated]]{
		AgentName:    agentName,
		ServiceName:  "cfm.agent." + agentName,
		ConfigPrefix: agentName,
		Subjects:     []string{subject},
		NewProcessor: func(*lifecycleagent.AgentContext) lifecycleagent.EventProcessor[lifecycleagent.CloudEvent[itemCreated]] {
			return proc
		},
	}

	shutdown := make(chan struct{})
	go lifecycleagent.LaunchAgent(shutdown, cfg)
	defer close(shutdown)

	// Wait for the durable consumer so that, under InterestPolicy, the message published below is retained.
	require.Eventually(t, func() bool {
		stream, err := nt.Client.JetStream.Stream(ctx, eventsStream)
		if err != nil {
			return false
		}
		_, err = stream.Consumer(ctx, agentName)
		return err == nil
	}, testTimeout, eventsPoll, "agent durable consumer should be created")

	event := lifecycleagent.CloudEvent[itemCreated]{
		SpecVersion:     lifecycleagent.SpecVersion,
		ID:              "evt-1",
		Source:          "/cfm/tmanager",
		Type:            "io.cfm.item.created",
		Subject:         "item/42",
		Time:            time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
		DataContentType: "application/json",
		Data:            itemCreated{ItemID: "42", Tenant: "suppliers"},
	}
	payload, err := json.Marshal(event)
	require.NoError(t, err)

	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, subject, payload)
	require.NoError(t, err)

	select {
	case evt := <-proc.received:
		assert.Equal(t, subject, evt.Subject)
		assert.Equal(t, lifecycleagent.SpecVersion, evt.Payload.SpecVersion)
		assert.Equal(t, "io.cfm.item.created", evt.Payload.Type)
		assert.Equal(t, "evt-1", evt.Payload.ID)
		assert.Equal(t, itemCreated{ItemID: "42", Tenant: "suppliers"}, evt.Payload.Data)
		assert.JSONEq(t, string(payload), string(evt.Raw))
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the CloudEvent to be dispatched")
	}
}
