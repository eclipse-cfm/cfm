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

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testActivity = "test-persist-activity"

func TestNatsActivityExecutor_ProcessingDataPersistedAcrossReschedules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, testStream)
	natsfixtures.SetupTestConsumer(t, ctx, stream, testActivity)

	msgClient := natsclient.NewMsgClient(nt.Client)

	// Create orchestration with single activity that will reschedule multiple times
	orchestration := api.Orchestration{
		ID:                "test-persist-data",
		CorrelationID:     "correlation-persist-test",
		State:             api.OrchestrationStateRunning,
		OrchestrationType: model.VPADeployType,
		ProcessingData:    make(map[string]any),
		OutputData:        make(map[string]any),
		Completed:         make(map[string]struct{}),
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "A1", Type: testActivity},
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

	// Create and start activity executor with a processor that reschedules three times
	// This tests that processing data is persisted correctly across multiple reschedules
	processor := &TestPersistenceActivityProcessor{}
	executor := &NatsActivityExecutor{
		Client:            msgClient,
		StreamName:        testStream,
		ActivityType:      testActivity,
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
			Type: testActivity,
		},
	}

	msgData, err := json.Marshal(activityMessage)
	require.NoError(t, err)

	subject := natsclient.CFMSubjectPrefix + "." + testActivity
	_, err = msgClient.Publish(ctx, subject, msgData)
	require.NoError(t, err)

	// Wait for orchestration response to be published
	select {
	case <-responseCaptured:
		// Response was captured successfully
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for orchestration response after multiple reschedules")
	}

	// Verify the orchestration response
	responseMutex.Lock()
	require.NotNil(t, capturedResponse, "Orchestration response should have been captured after multiple reschedules")
	assert.Equal(t, orchestration.ID, capturedResponse.ManifestID, "ManifestID should match orchestration ID")
	assert.True(t, capturedResponse.Success, "Response should indicate success")
	responseMutex.Unlock()

	// Verify the orchestration completed successfully
	updatedOrchestration, _, err := ReadOrchestration(ctx, orchestration.ID, msgClient)
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateCompleted, updatedOrchestration.State, "Orchestration should be completed")

	// CRITICAL: Verify that processing data was persisted correctly across all reschedules
	// The processor increments a counter on each invocation, and it should only complete
	// when the counter reaches 3 (after 2 reschedules)
	// If the bug exists (updating local variable instead of parameter in closure),
	// the counter won't be persisted and the processor will reschedule indefinitely
	assert.Equal(t, 3, processor.CallCount, "Processor should have been called exactly 3 times (initial + 2 reschedules)")
	assert.Equal(t, 2, processor.RescheduleCount, "Processor should have rescheduled exactly 2 times")
	assert.Equal(t, 1, processor.CompleteCount, "Processor should have completed exactly 1 time")

	// Verify that the counter was properly cleaned up from processing data
	_, counterExists := updatedOrchestration.ProcessingData["persist.counter"]
	assert.False(t, counterExists, "Counter should be deleted after completion")
}

// TestPersistenceActivityProcessor reschedules multiple times using a counter,
// testing that the counter value persists across reschedule boundaries
type TestPersistenceActivityProcessor struct {
	CallCount       int
	RescheduleCount int
	CompleteCount   int
}

func (p *TestPersistenceActivityProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	p.CallCount++

	// Get the counter from processing data
	counterVal, found := ctx.Value("persist.counter")
	var counter int
	if found {
		counter = int(counterVal.(float64))
	}

	counter++

	if counter >= 3 {
		// Third call - complete the activity
		ctx.Delete("persist.counter")
		p.CompleteCount++
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}

	// First or second call - increment counter and reschedule
	ctx.SetValue("persist.counter", float64(counter))
	p.RescheduleCount++
	return api.ActivityResult{
		Result:           api.ActivityResultSchedule,
		WaitOnReschedule: 10 * time.Millisecond,
	}
}
