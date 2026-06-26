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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout  = 30 * time.Second
	pollInterval = 100 * time.Millisecond
	bucket       = "cfm-bucket"
	streamName   = "events-stream"
	durableName  = "contract-definition-agent"
)

func TestContractDefinitionAgent_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, bucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	t.Setenv("CONTRACTDEFINITIONAGENT_URI", nt.URI)
	t.Setenv("CONTRACTDEFINITIONAGENT_BUCKET", bucket)
	t.Setenv("CONTRACTDEFINITIONAGENT_STREAM", streamName)

	shutdown := make(chan struct{})
	go LaunchAndWaitSignal(shutdown)
	defer close(shutdown)

	// The agent must create a durable consumer bound to the contract definition created subject.
	require.Eventually(t, func() bool {
		stream, err := nt.Client.JetStream.Stream(ctx, streamName)
		if err != nil {
			return false
		}
		consumer, err := stream.Consumer(ctx, durableName)
		if err != nil {
			return false
		}
		return assert.ObjectsAreEqual([]string{ContractDefinitionCreatedSubject}, consumer.CachedInfo().Config.FilterSubjects)
	}, testTimeout, pollInterval, "agent should create a durable consumer for the contract definition subject")

	// Publishing an event must be consumed and acknowledged (no pending messages remain).
	payload, _ := json.Marshal(map[string]string{"id": "cd-1", "participantContextId": "participant-1"})
	_, err = natsclient.NewMsgClient(nt.Client).Publish(ctx, ContractDefinitionCreatedSubject, payload)
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
