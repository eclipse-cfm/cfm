//  Copyright (c) 2025 Metaform Systems, Inc
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

package natsclient_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

// TestFetchLoop_RecoversFromDeletedConsumer verifies against a real JetStream server that deleting the consumer is
// detected by the loop and that it keeps processing messages through the recreated consumer.
func TestFetchLoop_RecoversFromDeletedConsumer(t *testing.T) {
	ctx := t.Context()
	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(context.Background(), nt)

	const subject = "loop-test"
	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, "cfm-stream")
	consumer := natsfixtures.SetupTestConsumer(t, ctx, stream, subject)
	info, err := consumer.Info(ctx)
	require.NoError(t, err)

	var recreated atomic.Int32
	loop := &natsclient.FetchLoop{
		RetryBackoff: 10 * time.Millisecond,
		Recreate: func(ctx context.Context) (jetstream.Consumer, error) {
			recreated.Add(1)
			return natsclient.SetupConsumer(ctx, stream, subject)
		},
	}

	// Delivery is at-least-once: a message acked just before the deletion can be redelivered to the recreated
	// consumer, so track which messages were handled rather than how often.
	var handled sync.Map
	handledSeq := func(seq uint64) bool { _, ok := handled.Load(seq); return ok }
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- loop.Run(loopCtx, consumer, printMonitor{t: t}, func(_ context.Context, msg jetstream.Msg) {
			if md, err := msg.Metadata(); err == nil {
				handled.Store(md.Sequence.Stream, true)
			}
			_ = msg.Ack()
		})
	}()

	publish := func() {
		_, err := nt.Client.JetStream.Publish(ctx, natsclient.CFMSubjectPrefix+"."+subject, []byte("{}"))
		require.NoError(t, err)
	}

	publish()
	require.Eventually(t, func() bool { return handledSeq(1) }, 10*time.Second, 50*time.Millisecond)

	require.NoError(t, stream.DeleteConsumer(ctx, info.Name))
	require.Eventually(t, func() bool { return recreated.Load() >= 1 }, 10*time.Second, 50*time.Millisecond,
		"the loop must detect the deleted consumer and recreate it")

	publish()
	require.Eventually(t, func() bool { return handledSeq(2) }, 10*time.Second, 50*time.Millisecond,
		"messages published after the deletion must be processed through the recreated consumer")
	require.NoError(t, loop.Check(time.Minute))

	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
}

// TestFetchLoop_HeartbeatPreventsRedelivery verifies against a real JetStream server that a handler running longer than
// the consumer's AckWait keeps its message: without the heartbeat, the message would be redelivered while the handler
// is still working on it.
func TestFetchLoop_HeartbeatPreventsRedelivery(t *testing.T) {
	ctx := t.Context()
	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(context.Background(), nt)

	const subject = "heartbeat-test"
	const ackWait = time.Second
	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, "cfm-stream")
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       ackWait,
		FilterSubject: natsclient.CFMSubjectPrefix + "." + subject,
	})
	require.NoError(t, err)

	loop := &natsclient.FetchLoop{HeartbeatInterval: ackWait / 4}
	var deliveries atomic.Int32
	var handled atomic.Bool
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- loop.Run(loopCtx, consumer, printMonitor{t: t}, func(_ context.Context, msg jetstream.Msg) {
			deliveries.Add(1)
			time.Sleep(3 * ackWait)
			_ = msg.Ack()
			handled.Store(true)
		})
	}()

	_, err = nt.Client.JetStream.Publish(ctx, natsclient.CFMSubjectPrefix+"."+subject, []byte("{}"))
	require.NoError(t, err)
	require.Eventually(t, handled.Load, 10*time.Second, 50*time.Millisecond)

	// Give a redelivery the chance to show up, then check the message was only delivered once
	time.Sleep(2 * ackWait)
	require.Equal(t, int32(1), deliveries.Load(), "a message that is still being handled must not be redelivered")
	info, err := consumer.Info(ctx)
	require.NoError(t, err)
	require.Zero(t, info.NumRedelivered)

	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
}

// printMonitor logs warnings to the test output.
type printMonitor struct {
	system.NoopMonitor
	t *testing.T
}

func (m printMonitor) Warnf(message string, args ...any) { m.t.Logf(message, args...) }
