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
	"errors"
	"strings"
	"testing"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// runResponseTest processes one message for a single-activity orchestration and returns the orchestration responses
// the executor published. updateErr, if set, makes every update of the orchestration fail.
func runResponseTest(t *testing.T, processor api.ActivityProcessor, engineState api.EngineState, updateErr error) []model.OrchestrationResponse {
	t.Helper()
	orchestration := api.Orchestration{
		ID:                "test-response",
		CorrelationID:     "profile-1",
		OrchestrationType: model.VPADeployType,
		State:             api.OrchestrationStateRunning,
		ProcessingData:    map[string]any{},
		OutputData:        map[string]any{},
		Completed:         map[string]struct{}{},
		EngineState:       engineState,
		Steps: []api.OrchestrationStep{
			{Activities: []api.Activity{{ID: "A1", Type: panicTestActivityType}}},
		},
	}
	orchestrationBytes, err := json.Marshal(orchestration)
	require.NoError(t, err)

	var responses []model.OrchestrationResponse
	mockClient := mocks.NewMockMsgClient(t)
	mockClient.EXPECT().Get(mock.Anything, "test-response").Return(panicTestEntry{value: orchestrationBytes}, nil)
	mockClient.EXPECT().Update(mock.Anything, "test-response", mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, _ []byte, revision uint64) (uint64, error) {
			if updateErr != nil {
				return 0, updateErr
			}
			return revision + 1, nil
		})
	mockClient.EXPECT().Publish(mock.Anything, natsclient.CFMOrchestrationResponseSubject, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, payload []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
			var response model.OrchestrationResponse
			require.NoError(t, json.Unmarshal(payload, &response))
			responses = append(responses, response)
			return &jetstream.PubAck{}, nil
		}).Maybe()

	executor := &NatsActivityExecutor{
		Client:            mockClient,
		ActivityType:      panicTestActivityType,
		ActivityProcessor: processor,
		Monitor:           system.NoopMonitor{},
	}
	messageBytes, err := json.Marshal(api.ActivityMessage{
		OrchestrationID: "test-response",
		Activity:        api.Activity{ID: "A1", Type: panicTestActivityType},
	})
	require.NoError(t, err)

	_ = executor.processMessage(t.Context(), &panicTestMsg{data: messageBytes, numDelivered: 1})
	return responses
}

func TestProcessMessage_FatalErrorIsReportedInResponse(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultFatalError, Error: errors.New("activity failed")}}

	responses := runResponseTest(t, processor, api.EngineState{}, nil)

	require.Len(t, responses, 1)
	assert.False(t, responses[0].Success)
	assert.Equal(t, "activity A1 of orchestration test-response failed", responses[0].ErrorDetail)
	assert.Equal(t, "activity failed", responses[0].ErrorCause)
	assert.Equal(t, "test-response", responses[0].ManifestID)
	assert.Equal(t, "profile-1", responses[0].CorrelationID)
}

func TestProcessMessage_ExhaustedRetriesAreReportedInResponse(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultRetryError, Error: errors.New("not ready")}}

	responses := runResponseTest(t, processor, api.EngineState{Retries: map[string]int{"A1": 9}}, nil)

	require.Len(t, responses, 1)
	assert.False(t, responses[0].Success)
	assert.Equal(t, "activity A1 of orchestration test-response failed", responses[0].ErrorDetail)
	assert.Equal(t, "activity A1 failed 10 times: not ready", responses[0].ErrorCause)
}

func TestProcessMessage_ErrorCauseInResponseIsTruncated(t *testing.T) {
	long := strings.Repeat("x", 3*maxPersistedErrorBytes)
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultFatalError, Error: errors.New(long)}}

	responses := runResponseTest(t, processor, api.EngineState{}, nil)

	require.Len(t, responses, 1)
	assert.Len(t, responses[0].ErrorCause, maxPersistedErrorBytes)
	assert.NotContains(t, responses[0].ErrorDetail, "xxx", "the detail for the caller does not quote the error")
}

// If the errored state cannot be recorded, the response must still report the failure and its reason.
func TestProcessMessage_FatalErrorIsReportedWhenStateUpdateFails(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultFatalError, Error: errors.New("activity failed")}}

	responses := runResponseTest(t, processor, api.EngineState{}, errors.New("kv unavailable"))

	require.Len(t, responses, 1)
	assert.False(t, responses[0].Success)
	assert.Equal(t, "activity A1 of orchestration test-response failed", responses[0].ErrorDetail)
	assert.Equal(t, "activity failed", responses[0].ErrorCause)
}

func TestProcessMessage_SuccessResponseHasNoErrorDetail(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultComplete}}

	responses := runResponseTest(t, processor, api.EngineState{}, nil)

	require.Len(t, responses, 1)
	assert.True(t, responses[0].Success)
	assert.Empty(t, responses[0].ErrorDetail)
	assert.Empty(t, responses[0].ErrorCause)
}
