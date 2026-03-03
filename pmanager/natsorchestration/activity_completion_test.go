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

package natsorchestration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/natsclient"
	"github.com/metaform/connector-fabric-manager/common/natsfixtures"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNatsActivityExecutor_OrchestrationResponsePublished(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, testStream)
	natsfixtures.SetupTestConsumer(t, ctx, stream, "test.response.activity")

	msgClient := natsclient.NewMsgClient(nt.Client)

	// Create orchestration with single activity that will complete the orchestration
	orchestration := api.Orchestration{
		ID:                "test-orchestration-response",
		State:             api.OrchestrationStateRunning,
		OrchestrationType: model.VPADeployType,
		ProcessingData:    make(map[string]any),
		OutputData:        make(map[string]any),
		Completed:         make(map[string]struct{}),
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "A1", Type: "test.response.activity"},
				},
			},
		},
	}

	serializedOrchestration, err := json.Marshal(orchestration)
	require.NoError(t, err)

	_, err = msgClient.Update(ctx, orchestration.ID, serializedOrchestration, 0)
	require.NoError(t, err)

	// Setup message capture for orchestration response
	var capturedResponse *model.OrchestrationResponse
	var responseMutex sync.Mutex
	responseCaptured := make(chan struct{})

	// Subscribe to orchestration response subject to capture the published message
	subscription, err := nt.Client.Connection.Subscribe(natsclient.CFMOrchestrationResponseSubject, func(msg *nats.Msg) {
		var response model.OrchestrationResponse
		if err := json.Unmarshal(msg.Data, &response); err == nil {
			responseMutex.Lock()
			capturedResponse = &response
			responseMutex.Unlock()
			close(responseCaptured)
		}
	})
	require.NoError(t, err)
	defer subscription.Unsubscribe()

	// Create and start activity executor with a processor that completes successfully
	processor := &TestCompleteActivityProcessor{}
	executor := &NatsActivityExecutor{
		Client:            msgClient,
		StreamName:        testStream,
		ActivityType:      "test.response.activity",
		ActivityProcessor: processor,
		Monitor:           system.NoopMonitor{},
	}

	err = executor.Execute(ctx)
	require.NoError(t, err)

	// Trigger orchestration by publishing activity message
	activityMessage := api.ActivityMessage{
		OrchestrationID: orchestration.ID,
		Activity: api.Activity{
			ID:   "A1",
			Type: "test.response.activity",
		},
	}

	msgData, err := json.Marshal(activityMessage)
	require.NoError(t, err)

	subject := natsclient.CFMSubjectPrefix + ".test-response-activity"
	_, err = msgClient.Publish(ctx, subject, msgData)
	require.NoError(t, err)

	// Wait for orchestration response to be published
	select {
	case <-responseCaptured:
		// Response was captured successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for orchestration response to be published")
	}

	// Verify the deployment response
	responseMutex.Lock()
	require.NotNil(t, capturedResponse, "Orchestration response should have been captured")
	assert.NotEmpty(t, capturedResponse.ID, "Response should have an ID")
	assert.Equal(t, orchestration.ID, capturedResponse.ManifestID, "ManifestID should match orchestration ID")
	assert.True(t, capturedResponse.Success, "Response should indicate success")
	assert.Equal(t, orchestration.OrchestrationType, capturedResponse.OrchestrationType, "OrchestrationType should match")
	assert.NotNil(t, capturedResponse.Properties, "Properties should be initialized")
	responseMutex.Unlock()
}

func TestNatsActivityExecutor_OrchestrationResponsePublishedOnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-activity-error-bucket")
	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, testStream)
	natsfixtures.SetupTestConsumer(t, ctx, stream, "test.error.activity")

	adapter := natsclient.NewMsgClient(nt.Client)

	// Create orchestration with single activity
	orchestration := api.Orchestration{
		ID:                "test-orchestration-error",
		State:             api.OrchestrationStateRunning,
		OrchestrationType: model.VPADeployType,
		ProcessingData:    make(map[string]any),
		Completed:         make(map[string]struct{}),
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "A1", Type: "test.error.activity"},
				},
			},
		},
	}

	// Store orchestration using the same method as the orchestrator
	serializedOrchestration, err := json.Marshal(orchestration)
	require.NoError(t, err)

	_, err = adapter.Update(ctx, orchestration.ID, serializedOrchestration, 0)
	require.NoError(t, err)

	// Setup message capture for orchestration response
	responseReceived := make(chan struct{})
	subscription, err := nt.Client.Connection.Subscribe(natsclient.CFMOrchestrationResponseSubject, func(msg *nats.Msg) {
		close(responseReceived)
	})
	require.NoError(t, err)
	defer subscription.Unsubscribe()

	// Create and start activity executor with a processor that returns fatal error
	processor := &TestFatalErrorActivityProcessor{}
	executor := &NatsActivityExecutor{
		Client:            adapter,
		StreamName:        testStream,
		ActivityType:      "test.error.activity",
		ActivityProcessor: processor,
		Monitor:           system.NoopMonitor{},
	}

	err = executor.Execute(ctx)
	require.NoError(t, err)

	// Trigger orchestration by publishing activity message
	activityMessage := api.ActivityMessage{
		OrchestrationID: orchestration.ID,
		Activity: api.Activity{
			ID:   "A1",
			Type: "test.error.activity",
		},
	}

	msgData, err := json.Marshal(activityMessage)
	require.NoError(t, err)

	subject := natsclient.CFMSubjectPrefix + ".test-error-activity"
	_, err = adapter.Publish(ctx, subject, msgData)
	require.NoError(t, err)

	// Wait and ensure no orchestration response is published
	select {
	case <-responseReceived:
		// Expected - no response should be published on error
	case <-time.After(2 * time.Second):
		t.Fatal("Orchestration response should not be published on fatal error")

	}

	// Verify orchestration is marked as errored
	updatedOrchestration, _, err := ReadOrchestration(ctx, orchestration.ID, adapter)
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateErrored, updatedOrchestration.State, "Orchestration should be in error state")
}

func TestNatsActivityExecutor_RescheduleWithCounter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-reschedule-bucket")
	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, testStream)
	natsfixtures.SetupTestConsumer(t, ctx, stream, "test.reschedule.activity")

	msgClient := natsclient.NewMsgClient(nt.Client)

	// Create orchestration with single activity that will reschedule once then complete
	orchestration := api.Orchestration{
		ID:                "test-reschedule-counter",
		CorrelationID:     "correlation-123",
		State:             api.OrchestrationStateRunning,
		OrchestrationType: model.VPADeployType,
		ProcessingData:    make(map[string]any),
		OutputData:        make(map[string]any),
		Completed:         make(map[string]struct{}),
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "A1", Type: "test.reschedule.activity"},
				},
			},
		},
	}

	serializedOrchestration, err := json.Marshal(orchestration)
	require.NoError(t, err)

	_, err = msgClient.Update(ctx, orchestration.ID, serializedOrchestration, 0)
	require.NoError(t, err)

	// Setup message capture for orchestration response
	var capturedResponse *model.OrchestrationResponse
	var responseMutex sync.Mutex
	responseCaptured := make(chan struct{})

	// Subscribe to orchestration response subject to capture the published message
	subscription, err := nt.Client.Connection.Subscribe(natsclient.CFMOrchestrationResponseSubject, func(msg *nats.Msg) {
		var dr model.OrchestrationResponse
		if err := json.Unmarshal(msg.Data, &dr); err == nil {
			responseMutex.Lock()
			capturedResponse = &dr
			responseMutex.Unlock()
			close(responseCaptured)
		}
	})
	require.NoError(t, err)
	defer subscription.Unsubscribe()

	// Create and start activity executor with a processor that reschedules once using a counter
	processor := &TestRescheduleCounterActivityProcessor{}
	executor := &NatsActivityExecutor{
		Client:            msgClient,
		StreamName:        testStream,
		ActivityType:      "test.reschedule.activity",
		ActivityProcessor: processor,
		Monitor:           system.NoopMonitor{},
	}

	err = executor.Execute(ctx)
	require.NoError(t, err)

	// Trigger orchestration by publishing activity message
	activityMessage := api.ActivityMessage{
		OrchestrationID: orchestration.ID,
		Activity: api.Activity{
			ID:   "A1",
			Type: "test.reschedule.activity",
		},
	}

	msgData, err := json.Marshal(activityMessage)
	require.NoError(t, err)

	subject := natsclient.CFMSubjectPrefix + ".test-reschedule-activity"
	_, err = msgClient.Publish(ctx, subject, msgData)
	require.NoError(t, err)

	// Wait for orchestration response to be published (should happen after reschedule and completion)
	select {
	case <-responseCaptured:
		// Response was captured successfully
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for orchestration response after reschedule")
	}

	// Verify the orchestration response
	responseMutex.Lock()
	require.NotNil(t, capturedResponse, "Orchestration response should have been captured after reschedule")
	assert.NotEmpty(t, capturedResponse.ID, "Response should have an ID")
	assert.Equal(t, orchestration.ID, capturedResponse.ManifestID, "ManifestID should match orchestration ID")
	assert.Equal(t, orchestration.CorrelationID, capturedResponse.CorrelationID, "CorrelationID should match")
	assert.True(t, capturedResponse.Success, "Response should indicate success")
	assert.Equal(t, orchestration.OrchestrationType, capturedResponse.OrchestrationType, "OrchestrationType should match")
	responseMutex.Unlock()

	// Verify the orchestration completed successfully
	updatedOrchestration, _, err := ReadOrchestration(ctx, orchestration.ID, msgClient)
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateCompleted, updatedOrchestration.State, "Orchestration should be completed")

	// Verify the counter was used and then deleted from processing data
	_, counterExists := updatedOrchestration.ProcessingData["dns.count"]
	assert.False(t, counterExists, "Counter should be deleted after completion")

	// Verify processor was called twice (once for reschedule, once for completion)
	assert.Equal(t, 2, processor.CallCount, "Processor should have been called twice")
}

// TestCompleteActivityProcessor always returns complete
type TestCompleteActivityProcessor struct{}

func (p *TestCompleteActivityProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{
		Result: api.ActivityResultComplete,
	}
}

// TestFatalErrorActivityProcessor always returns fatal error
type TestFatalErrorActivityProcessor struct{}

func (p *TestFatalErrorActivityProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{
		Result: api.ActivityResultFatalError,
		Error:  assert.AnError,
	}
}

// TestRescheduleCounterActivityProcessor reschedules once using a counter, then completes
type TestRescheduleCounterActivityProcessor struct {
	CallCount int
}

func (p *TestRescheduleCounterActivityProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	p.CallCount++

	count, found := ctx.Value("test.count")
	if found && count.(float64) > 0 {
		// Second call - complete the activity
		ctx.Delete("test.count")
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}

	// First call - set counter and reschedule
	ctx.SetValue("test.count", 1.0)
	return api.ActivityResult{
		Result:           api.ActivityResultSchedule,
		WaitOnReschedule: 10 * time.Millisecond,
	}
}
