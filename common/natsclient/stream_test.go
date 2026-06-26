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

package natsclient_test

import (
	"context"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const streamTestTimeout = 30 * time.Second

// provisionStream creates the event stream out-of-band, mirroring how the stream is provisioned in production (lifecycle
// agents never create it themselves).
func provisionStream(t *testing.T, ctx context.Context, nt *natsfixtures.NatsTestContainer, name string, subjects []string) jetstream.Stream {
	t.Helper()
	stream, err := nt.Client.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      name,
		Retention: jetstream.InterestPolicy,
		Storage:   jetstream.MemoryStorage,
		Subjects:  subjects,
	})
	require.NoError(t, err)
	return stream
}

func TestGetStream_CreateWhenStreamAbsent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	// The stream has not been provisioned, so resolving it must fail (the agent should then crash and be restarted).
	_, err = natsclient.GetStream(ctx, nt.Client, "events-stream", []string{"foo"})
	require.NoError(t, err)
}

func TestGetStream_ReturnsExistingStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	provisionStream(t, ctx, nt, "events-stream", []string{"events.contract.definition.created", "events.asset.>"})

	stream, err := natsclient.GetStream(ctx, nt.Client, "events-stream", nil)
	require.NoError(t, err)
	assert.Equal(t, "events-stream", stream.CachedInfo().Config.Name)
}

func TestEnsureStreamSubjects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	// Each subtest uses a distinct subject namespace: NATS prohibits overlapping subjects across streams in the same
	// account, not just within one stream.

	t.Run("subject covered by catch-all does not change the stream", func(t *testing.T) {
		provisionStream(t, ctx, nt, "covered-stream", []string{"covered.>"})
		stream, err := natsclient.GetStream(ctx, nt.Client, "covered-stream", nil)
		require.NoError(t, err)

		// Adding a specific subject must not produce "covered.>" + "covered.something" (which NATS would reject).
		updated, err := natsclient.EnsureStreamSubjects(ctx, nt.Client, stream, []string{"covered.contract.definition.created"})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"covered.>"}, updated.CachedInfo().Config.Subjects)
	})

	t.Run("disjoint subject is added to the stream", func(t *testing.T) {
		provisionStream(t, ctx, nt, "disjoint-stream", []string{"disjoint.contract.definition.created"})
		stream, err := natsclient.GetStream(ctx, nt.Client, "disjoint-stream", nil)
		require.NoError(t, err)

		updated, err := natsclient.EnsureStreamSubjects(ctx, nt.Client, stream, []string{"disjoint.asset.created"})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"disjoint.contract.definition.created", "disjoint.asset.created"},
			updated.CachedInfo().Config.Subjects)
	})
}

func TestSetupMultiSubjectConsumer_ReceivesOnEachSubject(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subjects := []string{"events.contract.definition.created", "events.contract.definition.updated"}
	provisionStream(t, ctx, nt, "events-stream", subjects)
	stream, err := natsclient.GetStream(ctx, nt.Client, "events-stream", nil)
	require.NoError(t, err)

	consumer, err := natsclient.SetupMultiSubjectConsumer(ctx, stream, "test-agent", subjects)
	require.NoError(t, err)

	client := natsclient.NewMsgClient(nt.Client)
	for _, subject := range subjects {
		_, err = client.Publish(ctx, subject, []byte(`{}`))
		require.NoError(t, err)
	}

	received := make(map[string]bool)
	assert.Eventually(t, func() bool {
		batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(time.Second))
		if err != nil {
			return false
		}
		for msg := range batch.Messages() {
			received[msg.Subject()] = true
			_ = msg.Ack()
		}
		return len(received) == len(subjects)
	}, streamTestTimeout, 100*time.Millisecond, "consumer should receive a message on each filter subject")
}

func TestSetupMultiSubjectConsumer_FanOutToMultipleConsumers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subject := "events.contract.definition.created"
	provisionStream(t, ctx, nt, "events-stream", []string{subject})
	stream, err := natsclient.GetStream(ctx, nt.Client, "events-stream", nil)
	require.NoError(t, err)

	// Two distinct agents (durable consumers) on the same subject. Under InterestPolicy each receives its own copy;
	// a WorkQueue stream would instead reject the second consumer / hand the message to only one of them.
	consumerA, err := natsclient.SetupMultiSubjectConsumer(ctx, stream, "agent-a", []string{subject})
	require.NoError(t, err)
	consumerB, err := natsclient.SetupMultiSubjectConsumer(ctx, stream, "agent-b", []string{subject})
	require.NoError(t, err)

	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, subject, []byte(`{}`))
	require.NoError(t, err)

	assert.True(t, consumerReceives(consumerA), "agent-a should receive the event")
	assert.True(t, consumerReceives(consumerB), "agent-b should receive the same event (fan-out)")
}

func consumerReceives(consumer jetstream.Consumer) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(500*time.Millisecond))
		if err != nil {
			continue
		}
		for msg := range batch.Messages() {
			_ = msg.Ack()
			return true
		}
	}
	return false
}
