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
	"testing"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// resultTestProcessor is an api.ActivityProcessor that returns a fixed result and records the values it was given.
type resultTestProcessor struct {
	result api.ActivityResult
	seen   map[string]any
}

func (p *resultTestProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	p.seen = make(map[string]any)
	for key, value := range ctx.Values() {
		p.seen[key] = value
	}
	return p.result
}

func (p *resultTestProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}

func (p *resultTestProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}

// spoofedPayload uses the keys under which the engine used to keep its own state in ProcessingData, with values of
// the wrong type.
var spoofedPayload = map[string]any{
	"trace_context":       1,
	"activity.A1.attempt": "many",
	"error":               "spoofed",
}

// runEngineStateTest processes one message for a single-activity orchestration whose ProcessingData holds
// spoofedPayload, and returns the message and every orchestration revision the executor wrote.
func runEngineStateTest(t *testing.T, processor api.ActivityProcessor, engineState api.EngineState) (*panicTestMsg, []api.Orchestration, error) {
	t.Helper()
	processingData := make(map[string]any)
	for key, value := range spoofedPayload {
		processingData[key] = value
	}
	orchestration := api.Orchestration{
		ID:             "test-engine-state",
		State:          api.OrchestrationStateRunning,
		ProcessingData: processingData,
		OutputData:     map[string]any{},
		Completed:      map[string]struct{}{},
		EngineState:    engineState,
		Steps: []api.OrchestrationStep{
			{Activities: []api.Activity{{ID: "A1", Type: panicTestActivityType}}},
		},
	}
	orchestrationBytes, err := json.Marshal(orchestration)
	require.NoError(t, err)

	var written []api.Orchestration
	mockClient := mocks.NewMockMsgClient(t)
	mockClient.EXPECT().Get(mock.Anything, "test-engine-state").Return(panicTestEntry{value: orchestrationBytes}, nil)
	mockClient.EXPECT().Update(mock.Anything, "test-engine-state", mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, value []byte, revision uint64) (uint64, error) {
			var o api.Orchestration
			require.NoError(t, json.Unmarshal(value, &o))
			written = append(written, o)
			return revision + 1, nil
		})
	mockClient.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything).Return(&jetstream.PubAck{}, nil)

	executor := &NatsActivityExecutor{
		Client:            mockClient,
		ActivityType:      panicTestActivityType,
		ActivityProcessor: processor,
		Monitor:           system.NoopMonitor{},
	}

	messageBytes, err := json.Marshal(api.ActivityMessage{
		OrchestrationID: "test-engine-state",
		Activity:        api.Activity{ID: "A1", Type: panicTestActivityType},
	})
	require.NoError(t, err)
	msg := &panicTestMsg{data: messageBytes}

	require.NotPanics(t, func() {
		err = executor.processMessage(t.Context(), msg)
	})
	return msg, written, err
}

func TestProcessMessage_PayloadCannotOverrideEngineState(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultComplete}}

	msg, written, err := runEngineStateTest(t, processor, api.EngineState{})

	require.NoError(t, err)
	assert.True(t, msg.acked)
	assert.False(t, msg.termed)

	// the payload reaches the activity as plain data
	assert.Equal(t, map[string]any{"trace_context": float64(1), "activity.A1.attempt": "many", "error": "spoofed"}, processor.seen)

	require.NotEmpty(t, written)
	last := written[len(written)-1]
	assert.Equal(t, api.OrchestrationStateCompleted, last.State)
	assert.Equal(t, map[string]int{"A1": 1}, last.EngineState.Attempts, "the attempt count must not be taken from the payload")
	assert.Empty(t, last.EngineState.Error, "the error must not be taken from the payload")
	assert.Empty(t, last.EngineState.TraceContext)
}

func TestProcessMessage_FatalErrorRecordedInEngineState(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultFatalError, Error: errors.New("activity failed")}}

	msg, written, err := runEngineStateTest(t, processor, api.EngineState{})

	require.ErrorContains(t, err, "activity failed")
	assert.True(t, msg.acked)

	require.Len(t, written, 1)
	assert.Equal(t, api.OrchestrationStateErrored, written[0].State)
	assert.Equal(t, "activity failed", written[0].EngineState.Error)
	assert.Equal(t, "spoofed", written[0].ProcessingData["error"], "the payload is left as it was")
}

func TestProcessMessage_AttemptCountedAcrossRedeliveries(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultComplete}}

	_, written, err := runEngineStateTest(t, processor, api.EngineState{Attempts: map[string]int{"A1": 2, "A0": 1}})

	require.NoError(t, err)
	require.NotEmpty(t, written)
	assert.Equal(t, map[string]int{"A0": 1, "A1": 3}, written[len(written)-1].EngineState.Attempts)
}
