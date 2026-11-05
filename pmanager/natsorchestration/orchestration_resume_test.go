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

package natsorchestration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resumeTestOrchestration(id string, state api.OrchestrationState) *api.Orchestration {
	return &api.Orchestration{
		ID:             id,
		State:          state,
		ProcessingData: map[string]any{},
		OutputData:     map[string]any{},
		Completed:      map[string]struct{}{},
		Steps: []api.OrchestrationStep{
			{Activities: []api.Activity{{ID: "A1", Type: "test.resume"}, {ID: "A2", Type: "test.resume"}}},
			{Activities: []api.Activity{{ID: "A3", Type: "test.resume"}}},
		},
	}
}

func setupResumeTest(t *testing.T) (context.Context, natsclient.MsgClient, jetstream.Stream) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	t.Cleanup(cancel)

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-resume-bucket")
	require.NoError(t, err)
	t.Cleanup(func() { natsfixtures.TeardownNatsContainer(context.Background(), nt) })

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, testStream)
	return ctx, natsclient.NewMsgClient(nt.Client), stream
}

// storeOrchestration writes the orchestration to the KV store without publishing any activity messages, as if the
// process died right after storing it.
func storeOrchestration(t *testing.T, ctx context.Context, client natsclient.MsgClient, orchestration *api.Orchestration) {
	serialized, err := json.Marshal(orchestration)
	require.NoError(t, err)
	_, err = client.Update(ctx, orchestration.ID, serialized, 0)
	require.NoError(t, err)
}

func streamMessages(t *testing.T, ctx context.Context, stream jetstream.Stream) uint64 {
	info, err := stream.Info(ctx)
	require.NoError(t, err)
	return info.State.Msgs
}

func TestExecute_MarksOrchestrationRunning(t *testing.T) {
	ctx, client, stream := setupResumeTest(t)
	orchestrator := NewNatsOrchestrator(client, system.NoopMonitor{})

	require.NoError(t, orchestrator.Execute(ctx, resumeTestOrchestration("O1", api.OrchestrationStateInitialized)))

	stored, _, err := ReadOrchestration(ctx, "O1", client)
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateRunning, stored.State)
	assert.Equal(t, uint64(2), streamMessages(t, ctx, stream), "the first step's activities must be enqueued")
}

func TestExecute_StartsOrchestrationThatWasStoredButNotStarted(t *testing.T) {
	ctx, client, stream := setupResumeTest(t)
	orchestrator := NewNatsOrchestrator(client, system.NoopMonitor{})
	storeOrchestration(t, ctx, client, resumeTestOrchestration("O1", api.OrchestrationStateInitialized))

	require.NoError(t, orchestrator.Execute(ctx, resumeTestOrchestration("O1", api.OrchestrationStateInitialized)))

	stored, _, err := ReadOrchestration(ctx, "O1", client)
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateRunning, stored.State)
	assert.Equal(t, uint64(2), streamMessages(t, ctx, stream), "the first step's activities must be enqueued")
}

func TestExecute_DoesNotRestartOrchestrationThatIsRunning(t *testing.T) {
	ctx, client, stream := setupResumeTest(t)
	orchestrator := NewNatsOrchestrator(client, system.NoopMonitor{})
	storeOrchestration(t, ctx, client, resumeTestOrchestration("O1", api.OrchestrationStateRunning))

	require.NoError(t, orchestrator.Execute(ctx, resumeTestOrchestration("O1", api.OrchestrationStateInitialized)))

	assert.Equal(t, uint64(0), streamMessages(t, ctx, stream), "a running orchestration must not be enqueued again")
}

func TestEnqueueActivityMessages_DropsDuplicates(t *testing.T) {
	ctx, client, stream := setupResumeTest(t)
	activities := resumeTestOrchestration("O1", api.OrchestrationStateInitialized).Steps[0].Activities

	require.NoError(t, EnqueueActivityMessages(ctx, "O1", activities, client))
	require.NoError(t, EnqueueActivityMessages(ctx, "O1", activities, client))

	assert.Equal(t, uint64(2), streamMessages(t, ctx, stream), "enqueueing the same activities again must not add messages")
}
