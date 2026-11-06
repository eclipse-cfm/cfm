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
	"time"
	"unicode/utf8"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/common/natsclient"
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
	return runEngineStateTestOnDelivery(t, processor, engineState, 1)
}

// runEngineStateTestOnDelivery is runEngineStateTest for a message that is on its numDelivered-th delivery.
func runEngineStateTestOnDelivery(t *testing.T, processor api.ActivityProcessor, engineState api.EngineState, numDelivered uint64) (*panicTestMsg, []api.Orchestration, error) {
	t.Helper()
	return runEngineStateTestWith(t, processor, engineState, numDelivered, nil)
}

// runEngineStateTestWith is runEngineStateTestOnDelivery with an executor that configure may adjust. It fails the test
// if the executor updates the orchestration with a context that is already done.
func runEngineStateTestWith(t *testing.T, processor api.ActivityProcessor, engineState api.EngineState, numDelivered uint64, configure func(*NatsActivityExecutor)) (*panicTestMsg, []api.Orchestration, error) {
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
		RunAndReturn(func(ctx context.Context, _ string, value []byte, revision uint64) (uint64, error) {
			require.NoError(t, ctx.Err(), "orchestration updates must not use an expired context")
			var o api.Orchestration
			require.NoError(t, json.Unmarshal(value, &o))
			written = append(written, o)
			return revision + 1, nil
		})
	mockClient.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything).Return(&jetstream.PubAck{}, nil).Maybe()

	executor := &NatsActivityExecutor{
		Client:            mockClient,
		ActivityType:      panicTestActivityType,
		ActivityProcessor: processor,
		Monitor:           system.NoopMonitor{},
	}
	if configure != nil {
		configure(executor)
	}

	messageBytes, err := json.Marshal(api.ActivityMessage{
		OrchestrationID: "test-engine-state",
		Activity:        api.Activity{ID: "A1", Type: panicTestActivityType},
	})
	require.NoError(t, err)
	msg := &panicTestMsg{data: messageBytes, numDelivered: numDelivered}

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

func TestProcessMessage_FatalErrorIsTruncatedInEngineState(t *testing.T) {
	long := strings.Repeat("x", 3*maxPersistedErrorBytes)
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultFatalError, Error: errors.New(long)}}

	_, written, err := runEngineStateTest(t, processor, api.EngineState{})

	require.Error(t, err)
	require.Len(t, written, 1)
	assert.Len(t, written[0].EngineState.Error, maxPersistedErrorBytes)
	assert.True(t, strings.HasSuffix(written[0].EngineState.Error, " (truncated)"))
}

func TestTruncateError(t *testing.T) {
	assert.Equal(t, "short", truncateError("short"))

	exact := strings.Repeat("x", maxPersistedErrorBytes)
	assert.Equal(t, exact, truncateError(exact))

	// A multibyte character that straddles the cut is dropped as a whole
	multibyte := truncateError(strings.Repeat("é", maxPersistedErrorBytes))
	assert.True(t, utf8.ValidString(multibyte))
	assert.LessOrEqual(t, len(multibyte), maxPersistedErrorBytes)
	assert.True(t, strings.HasSuffix(multibyte, " (truncated)"))
}

func TestProcessMessage_AttemptCountedAcrossRedeliveries(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultComplete}}

	_, written, err := runEngineStateTest(t, processor, api.EngineState{Attempts: map[string]int{"A1": 2, "A0": 1}})

	require.NoError(t, err)
	require.NotEmpty(t, written)
	assert.Equal(t, map[string]int{"A0": 1, "A1": 3}, written[len(written)-1].EngineState.Attempts)
}

func TestProcessMessage_RetryErrorUsesWaitOnReschedule(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{
		Result: api.ActivityResultRetryError, WaitOnReschedule: 30 * time.Second, Error: errors.New("not ready")}}

	msg, written, err := runEngineStateTest(t, processor, api.EngineState{})

	require.ErrorContains(t, err, "not ready")
	assert.True(t, msg.naked)
	assert.Equal(t, 30*time.Second, msg.nakDelay)
	assert.False(t, msg.acked)
	require.NotEmpty(t, written)
	assert.Equal(t, map[string]int{"A1": 1}, written[len(written)-1].EngineState.Retries)
}

func TestProcessMessage_RetryErrorBacksOff(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultRetryError, Error: errors.New("not ready")}}

	msg, written, err := runEngineStateTest(t, processor, api.EngineState{Retries: map[string]int{"A1": 2}})

	require.Error(t, err)
	assert.Equal(t, 5*time.Second, msg.nakDelay, "the third retry waits 5s")
	require.NotEmpty(t, written)
	assert.Equal(t, map[string]int{"A1": 3}, written[len(written)-1].EngineState.Retries)
}

// hangingProcessor blocks until its context is done and then fails fatally, as agents do for any downstream error.
type hangingProcessor struct {
	deadline    time.Time
	hasDeadline bool
}

func (p *hangingProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	p.deadline, p.hasDeadline = ctx.Context().Deadline()
	<-ctx.Context().Done()
	return api.ActivityResult{Result: api.ActivityResultFatalError, Error: ctx.Context().Err()}
}

func (p *hangingProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}
func (p *hangingProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}

// An activity that runs out of time is retried instead of failing the orchestration, and the executor can still record
// the outcome afterwards.
func TestProcessMessage_ActivityTimeoutIsRetried(t *testing.T) {
	processor := &hangingProcessor{}

	start := time.Now()
	msg, written, err := runEngineStateTestWith(t, processor, api.EngineState{}, 1, func(e *NatsActivityExecutor) {
		e.ActivityTimeout = 20 * time.Millisecond
	})

	require.ErrorContains(t, err, "did not finish within 20ms")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.True(t, processor.hasDeadline, "the activity must run with a deadline")
	assert.WithinDuration(t, start.Add(20*time.Millisecond), processor.deadline, time.Second)
	assert.True(t, msg.naked, "the activity must be retried")
	assert.False(t, msg.acked)
	require.NotEmpty(t, written)
	last := written[len(written)-1]
	assert.Equal(t, api.OrchestrationStateRunning, last.State)
	assert.Equal(t, map[string]int{"A1": 1}, last.EngineState.Retries)
}

// Retrying a timed-out activity is still bounded by the activity retry limit.
func TestProcessMessage_ActivityTimeoutGivesUpAfterMaxRetries(t *testing.T) {
	msg, written, err := runEngineStateTestWith(t, &hangingProcessor{}, api.EngineState{Retries: map[string]int{"A1": 9}}, 1, func(e *NatsActivityExecutor) {
		e.ActivityTimeout = 10 * time.Millisecond
	})

	require.ErrorContains(t, err, "activity A1 failed 10 times")
	assert.True(t, msg.acked)
	require.Len(t, written, 1)
	assert.Equal(t, api.OrchestrationStateErrored, written[0].State)
}

func TestProcessMessage_ActivityRunsWithDefaultTimeout(t *testing.T) {
	processor := &deadlineRecordingProcessor{}

	start := time.Now()
	_, _, err := runEngineStateTest(t, processor, api.EngineState{})

	require.NoError(t, err)
	require.True(t, processor.hasDeadline)
	assert.WithinDuration(t, start.Add(DefaultActivityTimeout), processor.deadline, time.Minute)
}

// deadlineRecordingProcessor completes immediately and records the deadline of its context.
type deadlineRecordingProcessor struct {
	deadline    time.Time
	hasDeadline bool
}

func (p *deadlineRecordingProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	p.deadline, p.hasDeadline = ctx.Context().Deadline()
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p *deadlineRecordingProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}

func (p *deadlineRecordingProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	return p.Process(ctx)
}

// Once an activity has used up its retries, the orchestration is marked errored instead of being retried forever.
func TestProcessMessage_RetryErrorGivesUpAfterMaxRetries(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultRetryError, Error: errors.New("not ready")}}

	msg, written, err := runEngineStateTest(t, processor, api.EngineState{Retries: map[string]int{"A1": 9}})

	require.ErrorContains(t, err, "activity A1 failed 10 times: not ready")
	assert.True(t, msg.acked)
	assert.False(t, msg.naked)
	require.Len(t, written, 1)
	assert.Equal(t, api.OrchestrationStateErrored, written[0].State)
	assert.Equal(t, "activity A1 failed 10 times: not ready", written[0].EngineState.Error)
}

// An activity message that reaches the consumer's delivery limit, for example because the activity reschedules forever,
// marks the orchestration errored instead of running the activity again.
func TestProcessMessage_FinalDeliveryMarksOrchestrationErrored(t *testing.T) {
	processor := &resultTestProcessor{result: api.ActivityResult{Result: api.ActivityResultSchedule, WaitOnReschedule: 5 * time.Second}}

	msg, written, err := runEngineStateTestOnDelivery(t, processor, api.EngineState{}, natsclient.ActivityMaxDeliver)

	require.ErrorContains(t, err, "delivered 1000 times without completing")
	assert.Nil(t, processor.seen, "the activity must not run again")
	assert.True(t, msg.acked)
	assert.False(t, msg.naked)
	require.Len(t, written, 1)
	assert.Equal(t, api.OrchestrationStateErrored, written[0].State)
}

func TestProcessMessage_FinalDeliveryWithoutOrchestrationTerminatesMessage(t *testing.T) {
	mockClient := mocks.NewMockMsgClient(t)
	mockClient.EXPECT().Get(mock.Anything, "missing").Return(nil, errors.New("kv unavailable"))
	executor := &NatsActivityExecutor{Client: mockClient, ActivityType: panicTestActivityType, Monitor: system.NoopMonitor{}}

	messageBytes, err := json.Marshal(api.ActivityMessage{OrchestrationID: "missing", Activity: api.Activity{ID: "A1", Type: panicTestActivityType}})
	require.NoError(t, err)

	msg := &panicTestMsg{data: messageBytes, numDelivered: natsclient.ActivityMaxDeliver - 1}
	require.Error(t, executor.processMessage(t.Context(), msg))
	assert.False(t, msg.termed, "earlier deliveries are left to redelivery")

	msg = &panicTestMsg{data: messageBytes, numDelivered: natsclient.ActivityMaxDeliver}
	require.ErrorContains(t, executor.processMessage(t.Context(), msg), "kv unavailable")
	assert.True(t, msg.termed)
}
