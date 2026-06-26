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

func TestSetupStreamWithSubjects_MultiTokenAndWildcard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	// Multi-token subjects and a wildcard subject, kept non-overlapping (a single stream rejects overlapping subjects).
	subjects := []string{"events.contract.definition.created", "events.asset.>"}

	// Creating the stream is idempotent.
	stream, err := natsclient.SetupStreamWithSubjects(ctx, nt.Client, "events-stream", subjects)
	require.NoError(t, err)
	_, err = natsclient.SetupStreamWithSubjects(ctx, nt.Client, "events-stream", subjects)
	require.NoError(t, err)

	info := stream.CachedInfo()
	assert.Equal(t, jetstream.InterestPolicy, info.Config.Retention)
	assert.ElementsMatch(t, subjects, info.Config.Subjects)
}

func TestSetupStreamWithSubjects_UnionOnUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	_, err = natsclient.SetupStreamWithSubjects(ctx, nt.Client, "events-stream", []string{"events.a.created"})
	require.NoError(t, err)

	// A second agent contributes another subject to the same stream; both must remain.
	stream, err := natsclient.SetupStreamWithSubjects(ctx, nt.Client, "events-stream", []string{"events.b.created"})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"events.a.created", "events.b.created"}, stream.CachedInfo().Config.Subjects)
}

func TestSetupMultiSubjectConsumer_ReceivesOnEachSubject(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subjects := []string{"events.contract.definition.created", "events.contract.definition.updated"}
	stream, err := natsclient.SetupStreamWithSubjects(ctx, nt.Client, "events-stream", subjects)
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

func TestSetupStreamWithSubjects_FanOutToMultipleConsumers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	subject := "events.contract.definition.created"
	stream, err := natsclient.SetupStreamWithSubjects(ctx, nt.Client, "events-stream", []string{subject})
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
