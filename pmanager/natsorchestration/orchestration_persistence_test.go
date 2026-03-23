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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	persistenceTimeout = 10 * time.Second
	pollInterval       = 10 * time.Millisecond
	streamName         = "cfm-activity"
)

func TestNatsOrchestrator_GetOrchestration_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-get-orchestration-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, streamName)
	natsfixtures.SetupTestConsumer(t, ctx, stream, "test.activity")

	adapter := natsclient.NewMsgClient(nt.Client)
	orchestrator := &NatsOrchestrator{
		Client:  adapter,
		monitor: system.NoopMonitor{},
	}

	// Create and execute an orchestration
	orchestration := &api.Orchestration{
		ID:             "test-get-orchestration-success",
		State:          api.OrchestrationStateRunning,
		Completed:      make(map[string]struct{}),
		ProcessingData: make(map[string]any),
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "A1", Type: "test.activity"},
				},
			},
		},
	}

	err = orchestrator.Execute(ctx, orchestration)
	require.NoError(t, err)

	// Test GetOrchestration
	result, err := orchestrator.GetOrchestration(ctx, orchestration.ID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, orchestration.ID, result.ID)
}

func TestNatsOrchestrator_GetOrchestration_NotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-get-orchestration-notfound-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	adapter := natsclient.NewMsgClient(nt.Client)
	orchestrator := &NatsOrchestrator{
		Client:  adapter,
		monitor: system.NoopMonitor{},
	}

	// Test GetOrchestration for non-existent orchestration
	result, err := orchestrator.GetOrchestration(ctx, "non-existent-orchestration")
	require.NoError(t, err)
	assert.Nil(t, result)
}

// Helper function to create test orchestration
func createTestOrchestration(id, activityType string) api.Orchestration {
	return api.Orchestration{
		ID:             id,
		State:          api.OrchestrationStateRunning,
		Completed:      make(map[string]struct{}),
		ProcessingData: make(map[string]any),
		OutputData:     make(map[string]any),
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "A1", Type: api.ActivityType(activityType)},
				},
			},
		},
	}
}

func Test_ValuePersistence(t *testing.T) {
	testCases := []struct {
		name          string
		bucketSuffix  string
		activityType  string
		setterFunc    func(ctx api.ActivityContext, key string, value any)
		validatorFunc func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any)
	}{
		{
			name:         "ProcessingData",
			bucketSuffix: "cfm-activity-context-bucket",
			activityType: "test.context.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.ProcessingData[key])
			},
		},
		{
			name:         "OutputData",
			bucketSuffix: "cfm-activity-output-context-bucket",
			activityType: "test.output.context.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetOutputValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.OutputData[key])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
			defer cancel()

			nt, err := natsfixtures.SetupNatsContainer(ctx, tc.bucketSuffix)
			require.NoError(t, err)
			defer natsfixtures.TeardownNatsContainer(ctx, nt)

			stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, streamName)
			natsfixtures.SetupTestConsumer(t, ctx, stream, tc.activityType)

			var wg sync.WaitGroup
			wg.Add(1)

			processor := &GenericValueProcessor{
				onProcess: func(activityCtx api.ActivityContext) {
					defer wg.Done()
					tc.setterFunc(activityCtx, "string_key", "test_value")
					tc.setterFunc(activityCtx, "int_key", 42)
					tc.setterFunc(activityCtx, "bool_key", true)
					tc.setterFunc(activityCtx, "map_key", map[string]any{
						"nested": "value",
						"count":  123,
					})
				},
			}

			orchestration := createTestOrchestration(fmt.Sprintf("test-%s-persistence", tc.name), tc.activityType)
			adapter := natsclient.NewMsgClient(nt.Client)

			orchestrator := &NatsOrchestrator{
				Client:  adapter,
				monitor: system.NoopMonitor{},
			}

			err = orchestrator.Execute(ctx, &orchestration)
			require.NoError(t, err)

			executor := &NatsActivityExecutor{
				Client:            adapter,
				StreamName:        "cfm-activity",
				ActivityType:      tc.activityType,
				ActivityProcessor: processor,
				Monitor:           system.NoopMonitor{},
			}

			err = executor.Execute(ctx)
			require.NoError(t, err)

			wg.Wait()

			// Verify values were persisted
			require.Eventually(t, func() bool {
				updatedOrchestration, _, err := ReadOrchestration(ctx, orchestration.ID, adapter)
				if err != nil {
					return false
				}

				// Check if all expected values are present
				dataMap := updatedOrchestration.ProcessingData
				if tc.name == "OutputData" {
					dataMap = updatedOrchestration.OutputData
				}

				if dataMap["string_key"] == nil {
					return false
				}

				tc.validatorFunc(t, &updatedOrchestration, "string_key", "test_value")
				tc.validatorFunc(t, &updatedOrchestration, "int_key", float64(42)) // JSON unmarshalling converts numbers to float64
				tc.validatorFunc(t, &updatedOrchestration, "bool_key", true)

				mapValue, ok := dataMap["map_key"].(map[string]any)
				require.True(t, ok, "map_key should be a map[string]any")
				assert.Equal(t, "value", mapValue["nested"])
				assert.Equal(t, float64(123), mapValue["count"])

				return true
			}, persistenceTimeout, pollInterval, "Values should be persisted")
		})
	}
}

func Test_ValuePersistenceOnRetry(t *testing.T) {
	testCases := []struct {
		name          string
		bucketSuffix  string
		activityType  string
		setterFunc    func(ctx api.ActivityContext, key string, value any)
		validatorFunc func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any)
	}{
		{
			name:         "ProcessingData",
			bucketSuffix: "cfm-activity-retry-bucket",
			activityType: "test.retry.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.ProcessingData[key])
			},
		},
		{
			name:         "OutputData",
			bucketSuffix: "cfm-activity-output-retry-bucket",
			activityType: "test.output.retry.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetOutputValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.OutputData[key])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
			defer cancel()

			nt, err := natsfixtures.SetupNatsContainer(ctx, tc.bucketSuffix)
			require.NoError(t, err)
			defer natsfixtures.TeardownNatsContainer(ctx, nt)

			stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, streamName)
			natsfixtures.SetupTestConsumer(t, ctx, stream, tc.activityType)

			var wg sync.WaitGroup
			var callCount int32

			processor := &GenericRetryProcessor{
				onProcess: func(activityCtx api.ActivityContext) api.ActivityResult {
					currentCall := atomic.AddInt32(&callCount, 1)

					if currentCall == 1 {
						// First call: set values and return retry error
						tc.setterFunc(activityCtx, "retry_count", int(currentCall))
						tc.setterFunc(activityCtx, "first_attempt_data", "initial_value")
						return api.ActivityResult{
							Result: api.ActivityResultRetryError,
							Error:  fmt.Errorf("simulated retry error"),
						}
					}

					// Second call: verify values from first call are available and set additional values
					tc.setterFunc(activityCtx, "retry_count", int(currentCall))
					tc.setterFunc(activityCtx, "second_attempt_data", "retry_value")
					wg.Done()
					return api.ActivityResult{
						Result: api.ActivityResultComplete,
					}
				},
			}

			orchestration := createTestOrchestration(fmt.Sprintf("test-%s-retry-persistence", tc.name), tc.activityType)
			adapter := natsclient.NewMsgClient(nt.Client)

			orchestrator := &NatsOrchestrator{
				Client:  adapter,
				monitor: system.NoopMonitor{},
			}

			err = orchestrator.Execute(ctx, &orchestration)
			require.NoError(t, err)

			executor := &NatsActivityExecutor{
				Client:            adapter,
				StreamName:        "cfm-activity",
				ActivityType:      tc.activityType,
				ActivityProcessor: processor,
				Monitor:           system.NoopMonitor{},
			}

			err = executor.Execute(ctx)
			require.NoError(t, err)

			wg.Add(1)
			wg.Wait()

			// Verify the processor was called twice
			assert.Equal(t, int32(2), atomic.LoadInt32(&callCount), "Processor should have been called twice")

			// Verify values were persisted
			require.Eventually(t, func() bool {
				finalOrchestration, _, err := ReadOrchestration(ctx, orchestration.ID, adapter)
				if err != nil {
					return false
				}

				dataMap := finalOrchestration.ProcessingData
				if tc.name == "OutputData" {
					dataMap = finalOrchestration.OutputData
				}

				if dataMap["retry_count"] == nil {
					return false
				}

				retryCount, ok := dataMap["retry_count"].(float64)
				if !ok || retryCount < 2 {
					return false
				}

				// Verify values from both attempts are present
				tc.validatorFunc(t, &finalOrchestration, "retry_count", float64(2))
				tc.validatorFunc(t, &finalOrchestration, "first_attempt_data", "initial_value")
				tc.validatorFunc(t, &finalOrchestration, "second_attempt_data", "retry_value")
				return true
			}, persistenceTimeout, pollInterval, "Retry values should be persisted")
		})
	}
}

func Test_ValuePersistenceMultipleActivities(t *testing.T) {
	testCases := []struct {
		name          string
		bucketSuffix  string
		activityType  string
		setterFunc    func(ctx api.ActivityContext, key string, value any)
		validatorFunc func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any)
	}{
		{
			name:         "ProcessingData",
			bucketSuffix: "cfm-multi-activity-bucket",
			activityType: "test.multi.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.ProcessingData[key])
			},
		},
		{
			name:         "OutputData",
			bucketSuffix: "cfm-multi-activity-output-bucket",
			activityType: "test.output.multi.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetOutputValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.OutputData[key])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
			defer cancel()

			nt, err := natsfixtures.SetupNatsContainer(ctx, tc.bucketSuffix)
			require.NoError(t, err)
			defer natsfixtures.TeardownNatsContainer(ctx, nt)

			stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, streamName)
			natsfixtures.SetupTestConsumer(t, ctx, stream, tc.activityType)

			var wg sync.WaitGroup
			wg.Add(2) // Two activities

			counter := &atomicCounter{}

			processor := &GenericValueProcessor{
				onProcess: func(activityCtx api.ActivityContext) {
					defer wg.Done()
					activityID := activityCtx.ID()
					tc.setterFunc(activityCtx, fmt.Sprintf("%s_key", activityID), fmt.Sprintf("value_from_%s", activityID))
					tc.setterFunc(activityCtx, "shared_counter", counter.IncrementAndGet())
				},
			}

			orchestration := api.Orchestration{
				ID:             fmt.Sprintf("test-multi-activity-%s-persistence", tc.name),
				State:          api.OrchestrationStateRunning,
				Completed:      make(map[string]struct{}),
				ProcessingData: make(map[string]any),
				OutputData:     make(map[string]any),
				Steps: []api.OrchestrationStep{
					{
						Activities: []api.Activity{
							{ID: "A1", Type: api.ActivityType(tc.activityType)},
							{ID: "A2", Type: api.ActivityType(tc.activityType)},
						},
					},
				},
			}

			adapter := natsclient.NewMsgClient(nt.Client)

			orchestrator := &NatsOrchestrator{
				Client:  adapter,
				monitor: system.NoopMonitor{},
			}

			err = orchestrator.Execute(ctx, &orchestration)
			require.NoError(t, err)

			// Create multiple executors
			for i := 0; i < 2; i++ {
				executor := &NatsActivityExecutor{
					Client:            adapter,
					StreamName:        "cfm-activity",
					ActivityType:      tc.activityType,
					ActivityProcessor: processor,
					Monitor:           system.NoopMonitor{},
				}
				err = executor.Execute(ctx)
				require.NoError(t, err)
			}

			wg.Wait()

			// Verify values were persisted
			require.Eventually(t, func() bool {
				finalOrchestration, _, err := ReadOrchestration(ctx, orchestration.ID, adapter)
				if err != nil {
					return false
				}

				dataMap := finalOrchestration.ProcessingData
				if tc.name == "OutputData" {
					dataMap = finalOrchestration.OutputData
				}

				if dataMap["A1_key"] == nil || dataMap["A2_key"] == nil {
					return false
				}

				// Verify values from both activities are present
				tc.validatorFunc(t, &finalOrchestration, "A1_key", "value_from_A1")
				tc.validatorFunc(t, &finalOrchestration, "A2_key", "value_from_A2")

				// Verify shared counter was handled properly
				_, exists := dataMap["shared_counter"]
				assert.True(t, exists, "shared_counter should exist")
				return true
			}, persistenceTimeout, pollInterval, "Multi-activity values should be persisted")
		})
	}
}

func Test_ValuePersistenceOnWait(t *testing.T) {
	testCases := []struct {
		name          string
		bucketSuffix  string
		activityType  string
		setterFunc    func(ctx api.ActivityContext, key string, value any)
		validatorFunc func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any)
	}{
		{
			name:         "ProcessingData",
			bucketSuffix: "cfm-wait-activity-bucket",
			activityType: "test.wait.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.ProcessingData[key])
			},
		},
		{
			name:         "OutputData",
			bucketSuffix: "cfm-wait-activity-output-bucket",
			activityType: "test.output.wait.persistence",
			setterFunc: func(ctx api.ActivityContext, key string, value any) {
				ctx.SetOutputValue(key, value)
			},
			validatorFunc: func(t *testing.T, orchestration *api.Orchestration, key string, expectedValue any) {
				assert.Equal(t, expectedValue, orchestration.OutputData[key])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
			defer cancel()

			nt, err := natsfixtures.SetupNatsContainer(ctx, tc.bucketSuffix)
			require.NoError(t, err)
			defer natsfixtures.TeardownNatsContainer(ctx, nt)

			stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, streamName)
			natsfixtures.SetupTestConsumer(t, ctx, stream, tc.activityType)

			var wg sync.WaitGroup
			wg.Add(1)

			processor := &GenericWaitProcessor{
				onProcess: func(activityCtx api.ActivityContext) {
					defer wg.Done()
					tc.setterFunc(activityCtx, "wait_state", "waiting")
					tc.setterFunc(activityCtx, "wait_timestamp", time.Now().Unix())
				},
			}

			orchestration := createTestOrchestration(fmt.Sprintf("test-%s-wait-persistence", tc.name), tc.activityType)
			adapter := natsclient.NewMsgClient(nt.Client)

			orchestrator := &NatsOrchestrator{
				Client:  adapter,
				monitor: system.NoopMonitor{},
			}

			err = orchestrator.Execute(ctx, &orchestration)
			require.NoError(t, err)

			executor := &NatsActivityExecutor{
				Client:            adapter,
				StreamName:        "cfm-activity",
				ActivityType:      tc.activityType,
				ActivityProcessor: processor,
				Monitor:           system.NoopMonitor{},
			}

			err = executor.Execute(ctx)
			require.NoError(t, err)

			wg.Wait()

			// Verify values were persisted
			require.Eventually(t, func() bool {
				waitOrchestration, _, err := ReadOrchestration(ctx, orchestration.ID, adapter)
				if err != nil {
					return false
				}

				dataMap := waitOrchestration.ProcessingData
				if tc.name == "OutputData" {
					dataMap = waitOrchestration.OutputData
				}

				if dataMap["wait_state"] == nil {
					return false
				}

				// Verify values were persisted during wait
				tc.validatorFunc(t, &waitOrchestration, "wait_state", "waiting")
				assert.NotNil(t, dataMap["wait_timestamp"])
				return true
			}, persistenceTimeout, pollInterval, "Wait values should be persisted")
		})
	}
}

// Generic test processors

type GenericValueProcessor struct {
	DefaultTestProcessor
	onProcess func(api.ActivityContext)
}

func (p *GenericValueProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if p.onProcess != nil {
		p.onProcess(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

type GenericRetryProcessor struct {
	DefaultTestProcessor
	onProcess func(api.ActivityContext) api.ActivityResult
}

func (p *GenericRetryProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if p.onProcess != nil {
		return p.onProcess(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

type GenericWaitProcessor struct {
	DefaultTestProcessor
	onProcess func(api.ActivityContext)
}

func (p *GenericWaitProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if p.onProcess != nil {
		p.onProcess(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultWait}
}

// Test processors

type ValueSettingProcessor struct {
	DefaultTestProcessor
	onProcess func(api.ActivityContext)
}

func (p *ValueSettingProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if p.onProcess != nil {
		p.onProcess(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

type RetryWithValueProcessor struct {
	DefaultTestProcessor
	onProcess func(api.ActivityContext) api.ActivityResult
}

func (p *RetryWithValueProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if p.onProcess != nil {
		return p.onProcess(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

type MultiActivityValueProcessor struct {
	DefaultTestProcessor
	onProcess func(api.ActivityContext)
}

func (p *MultiActivityValueProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if p.onProcess != nil {
		p.onProcess(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

type WaitWithValueProcessor struct {
	DefaultTestProcessor
	onProcess func(api.ActivityContext)
}

func (p *WaitWithValueProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if p.onProcess != nil {
		p.onProcess(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultWait}
}

// Thread-safe atomic counter
type atomicCounter struct {
	count int64
}

func (c *atomicCounter) IncrementAndGet() int {
	return int(atomic.AddInt64(&c.count, 1))
}
