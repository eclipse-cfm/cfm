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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// revisionConflict is the error the KV store returns when an entry changed since the revision the update was based on.
var revisionConflict = &jetstream.APIError{Code: 400, ErrorCode: jetstream.JSErrCodeStreamWrongLastSequence, Description: "wrong last sequence"}

func clientTestOrchestration(t *testing.T, id string) (api.Orchestration, []byte) {
	orchestration := api.Orchestration{
		ID:             id,
		ProcessingData: map[string]any{},
		OutputData:     map[string]any{},
		Completed:      map[string]struct{}{},
		Steps: []api.OrchestrationStep{
			{Activities: []api.Activity{{ID: "A1", Type: panicTestActivityType}}},
		},
	}
	serialized, err := json.Marshal(orchestration)
	require.NoError(t, err)
	return orchestration, serialized
}

func TestUpdateOrchestration_ReturnsNonConflictErrors(t *testing.T) {
	orchestration, _ := clientTestOrchestration(t, "test-update")

	mockClient := mocks.NewMockMsgClient(t)
	mockClient.EXPECT().Update(mock.Anything, "test-update", mock.Anything, uint64(1)).
		Return(uint64(0), errors.New("maximum bytes exceeded")).Once()

	_, _, err := UpdateOrchestration(t.Context(), orchestration, 1, mockClient, func(*api.Orchestration) {})

	require.ErrorContains(t, err, "maximum bytes exceeded")
}

func TestUpdateOrchestration_RetriesConflicts(t *testing.T) {
	orchestration, serialized := clientTestOrchestration(t, "test-update")

	mockClient := mocks.NewMockMsgClient(t)
	mockClient.EXPECT().Update(mock.Anything, "test-update", mock.Anything, uint64(1)).
		Return(uint64(0), revisionConflict).Once()
	mockClient.EXPECT().Get(mock.Anything, "test-update").Return(panicTestEntry{value: serialized}, nil).Once()
	mockClient.EXPECT().Update(mock.Anything, "test-update", mock.Anything, uint64(1)).
		Return(uint64(2), nil).Once()

	calls := 0
	updated, revision, err := UpdateOrchestration(t.Context(), orchestration, 1, mockClient, func(o *api.Orchestration) {
		calls++
		o.SetState(api.OrchestrationStateRunning)
	})

	require.NoError(t, err)
	assert.Equal(t, 2, calls, "the update must be applied again to the re-read orchestration")
	assert.Equal(t, uint64(2), revision, "the revision written by the update must be returned")
	assert.Equal(t, api.OrchestrationStateRunning, updated.State)
}

func TestUpdateOrchestration_GivesUpAfterRepeatedConflicts(t *testing.T) {
	orchestration, serialized := clientTestOrchestration(t, "test-update")

	mockClient := mocks.NewMockMsgClient(t)
	mockClient.EXPECT().Update(mock.Anything, "test-update", mock.Anything, uint64(1)).
		Return(uint64(0), revisionConflict).Times(maxUpdateConflicts)
	mockClient.EXPECT().Get(mock.Anything, "test-update").Return(panicTestEntry{value: serialized}, nil).
		Times(maxUpdateConflicts - 1)

	_, _, err := UpdateOrchestration(t.Context(), orchestration, 1, mockClient, func(*api.Orchestration) {})

	require.ErrorContains(t, err, "conflicting updates")
}

func TestProcessMessage_PersistFailureRedeliversMessage(t *testing.T) {
	tests := []struct {
		name   string
		result api.ActivityResult
	}{
		{name: "wait", result: api.ActivityResult{Result: api.ActivityResultWait}},
		{name: "schedule", result: api.ActivityResult{Result: api.ActivityResultSchedule, WaitOnReschedule: time.Hour}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, serialized := clientTestOrchestration(t, "test-persist")

			mockClient := mocks.NewMockMsgClient(t)
			mockClient.EXPECT().Get(mock.Anything, "test-persist").Return(panicTestEntry{value: serialized}, nil)
			mockClient.EXPECT().Update(mock.Anything, "test-persist", mock.Anything, uint64(1)).
				Return(uint64(0), errors.New("maximum bytes exceeded"))

			executor := &NatsActivityExecutor{
				Client:            mockClient,
				ActivityType:      panicTestActivityType,
				ActivityProcessor: &resultTestProcessor{result: tt.result},
				Monitor:           system.NoopMonitor{},
			}

			messageBytes, err := json.Marshal(api.ActivityMessage{
				OrchestrationID: "test-persist",
				Activity:        api.Activity{ID: "A1", Type: panicTestActivityType},
			})
			require.NoError(t, err)
			msg := &panicTestMsg{data: messageBytes, numDelivered: 1}

			err = executor.processMessage(t.Context(), msg)

			require.ErrorContains(t, err, "maximum bytes exceeded")
			assert.False(t, msg.acked, "a message whose state could not be saved must not be acked")
			assert.True(t, msg.naked, "a message whose state could not be saved must be redelivered")
			assert.NotEqual(t, time.Hour, msg.nakDelay, "the redelivery must not wait for the reschedule delay")
		})
	}
}

// failingProcessor is an api.ActivityProcessor that fails the test when it is called.
type failingProcessor struct {
	t *testing.T
}

func (p failingProcessor) Process(api.ActivityContext) api.ActivityResult {
	p.t.Error("a completed activity must not run again")
	return api.ActivityResult{Result: api.ActivityResultComplete}
}
func (p failingProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}
func (p failingProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}

func TestProcessMessage_CompletedActivityIsNotRunAgain(t *testing.T) {
	tests := []struct {
		name          string
		state         api.OrchestrationState
		expectNextRun bool
	}{
		{name: "running orchestration continues with the next step", state: api.OrchestrationStateRunning, expectNextRun: true},
		{name: "completed orchestration is left alone", state: api.OrchestrationStateCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestration := api.Orchestration{
				ID:             "test-completed",
				State:          tt.state,
				ProcessingData: map[string]any{},
				OutputData:     map[string]any{},
				Completed:      map[string]struct{}{"A1": {}},
				Steps: []api.OrchestrationStep{
					{Activities: []api.Activity{{ID: "A1", Type: panicTestActivityType}}},
					{Activities: []api.Activity{{ID: "A2", Type: panicTestActivityType}}},
				},
			}
			serialized, err := json.Marshal(orchestration)
			require.NoError(t, err)

			mockClient := mocks.NewMockMsgClient(t)
			mockClient.EXPECT().Get(mock.Anything, "test-completed").Return(panicTestEntry{value: serialized}, nil)
			if tt.expectNextRun {
				mockClient.EXPECT().Update(mock.Anything, "test-completed", mock.Anything, uint64(1)).Return(uint64(2), nil)
				mockClient.EXPECT().PublishMsg(mock.Anything, mock.MatchedBy(func(msg *nats.Msg) bool {
					return msg.Header.Get(jetstream.MsgIDHeader) == ActivityMessageID("test-completed", "A2")
				})).Return(&jetstream.PubAck{}, nil).Once()
			}

			executor := &NatsActivityExecutor{
				Client:            mockClient,
				ActivityType:      panicTestActivityType,
				ActivityProcessor: failingProcessor{t: t},
				Monitor:           system.NoopMonitor{},
			}

			messageBytes, err := json.Marshal(api.ActivityMessage{
				OrchestrationID: "test-completed",
				Activity:        api.Activity{ID: "A1", Type: panicTestActivityType},
			})
			require.NoError(t, err)
			msg := &panicTestMsg{data: messageBytes, numDelivered: 2}

			require.NoError(t, executor.processMessage(t.Context(), msg))
			assert.True(t, msg.acked)
		})
	}
}
