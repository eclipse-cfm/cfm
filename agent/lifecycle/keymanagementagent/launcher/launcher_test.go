/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package launcher

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout  = 30 * time.Second
	pollInterval = 100 * time.Millisecond
	bucket       = "cfm-bucket"
	streamName   = "events-stream"
	durableName  = "key-management-agent"
)

func TestKeyManagementAgent_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, bucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	t.Setenv("KMAGENT_URI", nt.URI)
	t.Setenv("KMAGENT_BUCKET", bucket)
	t.Setenv("KMAGENT_STREAM", streamName)

	// The event stream is provisioned out-of-band; the agent only binds a consumer to it and must fail to start if it
	// is absent.
	_, err = nt.Client.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      streamName,
		Retention: jetstream.InterestPolicy,
		Storage:   jetstream.MemoryStorage,
		Subjects:  []string{KeyPairSubject},
	})
	require.NoError(t, err)

	shutdown := make(chan struct{})
	go LaunchAndWaitSignal(shutdown)
	defer close(shutdown)

	// The agent must create a durable consumer bound to the key management subjects.
	require.Eventually(t, func() bool {
		stream, err := nt.Client.JetStream.Stream(ctx, streamName)
		if err != nil {
			return false
		}
		_, err = stream.Consumer(ctx, durableName)
		return err == nil
	}, testTimeout, pollInterval, "agent should create a durable consumer for the key management subjects")

	stream, err := nt.Client.JetStream.Stream(ctx, streamName)
	require.NoError(t, err)
	consumer, err := stream.Consumer(ctx, durableName)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{KeyPairSubject}, consumer.CachedInfo().Config.FilterSubjects)

	// Publishing a CloudEvent must be consumed and acknowledged (no pending messages remain).
	payload, _ := json.Marshal(map[string]any{
		"specversion": "1.0",
		"id":          "ce-1",
		"source":      "/cfm/test",
		"type":        "io.cfm.key.management.created",
		"data":        map[string]string{"id": "km-1", "participantContextId": "participant-1"},
	})
	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, "events.keypair.created", payload)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		stream, err := nt.Client.JetStream.Stream(ctx, streamName)
		if err != nil {
			return false
		}
		consumer, err := stream.Consumer(ctx, durableName)
		if err != nil {
			return false
		}
		return consumer.CachedInfo().NumPending == 0 && consumer.CachedInfo().NumAckPending == 0
	}, testTimeout, pollInterval, "published event should be consumed and acknowledged")
}
