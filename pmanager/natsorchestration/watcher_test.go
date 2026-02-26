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
	"testing"
	"time"

	"github.com/metaform/connector-fabric-manager/common/memorystore"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/metaform/connector-fabric-manager/pmanager/api/mocks"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Successfully create a new orchestration entry
func TestOnMessage_CreateNewEntry(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	msg := createNatsMsg(t, orch)

	watcher.onMessage(msg.Data, msg)

	ctx := context.Background()
	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, "orch-1", entry.ID)
	assert.Equal(t, "corr-1", entry.CorrelationID)
	assert.Equal(t, api.OrchestrationStateRunning, entry.State)
}

// Update existing entry with new state
func TestOnMessage_UpdateExistingEntry(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	// Create initial entry
	orch1 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	entry1 := convertToEntry(orch1)
	_, err := index.Create(ctx, entry1)
	require.NoError(t, err)

	// Update to new state
	orch2 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateCompleted)
	orch2.StateTimestamp = time.Now()
	msg := createNatsMsg(t, orch2)

	watcher.onMessage(msg.Data, msg)

	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateCompleted, entry.State)
}

// Handle malformed JSON gracefully
func TestOnMessage_MalformedJSON(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	msg := &nats.Msg{Data: []byte("invalid json")}

	watcher.onMessage(msg.Data, msg)

	// Should not crash, message handling completed
	ctx := context.Background()
	_, err := index.FindByID(ctx, "")
	// Entry should not exist, error is expected
	assert.Error(t, err)
}

// Don't update if current state equals new state
func TestOnMessage_UpdateWhenStateUnchangedTimestampDifferent(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	// Create initial entry in Running state
	orch1 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	initialTime := time.Now()
	orch1.StateTimestamp = initialTime
	entry1 := convertToEntry(orch1)
	_, err := index.Create(ctx, entry1)
	require.NoError(t, err)

	// Update with same state but different timestamp
	orch2 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	orch2.StateTimestamp = time.Now().Add(1 * time.Second)
	msg := createNatsMsg(t, orch2)

	watcher.onMessage(msg.Data, msg)

	// Verify state timestamp didn't change
	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.True(t, entry.StateTimestamp.Equal(orch2.StateTimestamp))
}

// Don't update if already in Completed state
func TestOnMessage_IgnoreUpdatesWhenCompleted(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	// Create entry in Completed state
	orch1 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateCompleted)
	completedTime := time.Now()
	orch1.StateTimestamp = completedTime
	entry1 := convertToEntry(orch1)
	_, err := index.Create(ctx, entry1)
	require.NoError(t, err)

	// Try to update to Running state (out of order message)
	orch2 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	orch2.StateTimestamp = time.Now().Add(-10 * time.Second)
	msg := createNatsMsg(t, orch2)

	watcher.onMessage(msg.Data, msg)

	// Verify state remains Completed
	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateCompleted, entry.State)
}

// Update if new state is Errored and current state is not Errored
func TestOnMessage_ErroredStateUpdate(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	pManager := &mocks.MockProvisionManager{}
	pManager.On("Start", mock.Anything, mock.Anything).Return(&api.Orchestration{}, nil)
	watcher := createTestWatcher(index, trxContext, pManager)

	ctx := context.Background()

	// Create entry in Running state
	orch1 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	runningTime := time.Now()
	orch1.StateTimestamp = runningTime
	entry1 := convertToEntry(orch1)
	_, err := index.Create(ctx, entry1)
	require.NoError(t, err)

	// Try to update to Errored state
	orch2 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateErrored)
	orch2.StateTimestamp = time.Now()
	msg := createNatsMsg(t, orch2)

	watcher.onMessage(msg.Data, msg)

	// Verify state remains Running
	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateErrored, entry.State)
}

// Create multiple independent entries
func TestOnMessage_CreateMultipleEntries(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	orch1 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	msg1 := createNatsMsg(t, orch1)
	watcher.onMessage(msg1.Data, msg1)

	orch2 := createWatcherOrchestration("orch-2", "corr-2", api.OrchestrationStateRunning)
	msg2 := createNatsMsg(t, orch2)
	watcher.onMessage(msg2.Data, msg2)

	orch3 := createWatcherOrchestration("orch-3", "corr-3", api.OrchestrationStateRunning)
	msg3 := createNatsMsg(t, orch3)
	watcher.onMessage(msg3.Data, msg3)

	// Verify all entries exist
	entry1, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, "corr-1", entry1.CorrelationID)

	entry2, err := index.FindByID(ctx, "orch-2")
	require.NoError(t, err)
	assert.Equal(t, "corr-2", entry2.CorrelationID)

	entry3, err := index.FindByID(ctx, "orch-3")
	require.NoError(t, err)
	assert.Equal(t, "corr-3", entry3.CorrelationID)
}

// Entry has correct metadata after creation
func TestOnMessage_CorrectMetadataAfterCreate(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	now := time.Now()
	orch := api.Orchestration{
		ID:                "orch-1",
		CorrelationID:     "corr-1",
		State:             api.OrchestrationStateRunning,
		StateTimestamp:    now,
		CreatedTimestamp:  now.Add(-1 * time.Minute),
		OrchestrationType: "TestWorkflow",
		Steps:             []api.OrchestrationStep{},
		ProcessingData:    map[string]any{"key": "value"},
		OutputData:        map[string]any{},
		Completed:         map[string]struct{}{},
	}
	msg := createNatsMsg(t, orch)

	watcher.onMessage(msg.Data, msg)

	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, "orch-1", entry.ID)
	assert.Equal(t, "corr-1", entry.CorrelationID)
	assert.Equal(t, api.OrchestrationStateRunning, entry.State)
	assert.Equal(t, now.Unix(), entry.StateTimestamp.Unix())
	assert.Equal(t, now.Add(-1*time.Minute).Unix(), entry.CreatedTimestamp.Unix())
	assert.Equal(t, model.OrchestrationType("TestWorkflow"), entry.OrchestrationType)
}

// Transition from Initialized to Running to Completed
func TestOnMessage_StateTransitionSequence(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	// Step 1: Create in Initialized state
	orch1 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateInitialized)
	msg1 := createNatsMsg(t, orch1)
	watcher.onMessage(msg1.Data, msg1)

	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateInitialized, entry.State)

	// Step 2: Transition to Running
	orch2 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	orch2.StateTimestamp = time.Now().Add(1 * time.Second)
	msg2 := createNatsMsg(t, orch2)
	watcher.onMessage(msg2.Data, msg2)

	entry, err = index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateRunning, entry.State)

	// Step 3: Transition to Completed
	orch3 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateCompleted)
	orch3.StateTimestamp = time.Now().Add(2 * time.Second)
	msg3 := createNatsMsg(t, orch3)
	watcher.onMessage(msg3.Data, msg3)

	entry, err = index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateCompleted, entry.State)
}

// Multiple state transitions for same orchestration
func TestOnMessage_MultipleUpdatesToSameEntry(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	baseTime := time.Now()

	// Multiple state changes
	states := []api.OrchestrationState{
		api.OrchestrationStateInitialized,
		api.OrchestrationStateRunning,
		api.OrchestrationStateCompleted,
	}

	for i, state := range states {
		orch := createWatcherOrchestration("orch-1", "corr-1", state)
		orch.StateTimestamp = baseTime.Add(time.Duration(i) * time.Second)
		msg := createNatsMsg(t, orch)
		watcher.onMessage(msg.Data, msg)
	}

	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, api.OrchestrationStateCompleted, entry.State)
}

// Correlation ID is preserved
func TestOnMessage_CorrelationIDPreserved(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	msg := createNatsMsg(t, orch)
	watcher.onMessage(msg.Data, msg)

	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, "corr-1", entry.CorrelationID)
}

// Verify entry properties preserved across updates
func TestOnMessage_EntryPropertiesPreserved(t *testing.T) {
	index := createTestStore(t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(index, trxContext, &mocks.MockProvisionManager{})

	ctx := context.Background()

	createdTime := time.Now().Add(-10 * time.Minute)

	orch1 := api.Orchestration{
		ID:                "orch-1",
		CorrelationID:     "corr-1",
		State:             api.OrchestrationStateRunning,
		StateTimestamp:    time.Now(),
		CreatedTimestamp:  createdTime,
		OrchestrationType: "MyWorkflow",
		Steps:             []api.OrchestrationStep{},
		ProcessingData:    map[string]any{},
		OutputData:        map[string]any{},
		Completed:         map[string]struct{}{},
	}
	msg1 := createNatsMsg(t, orch1)
	watcher.onMessage(msg1.Data, msg1)

	// Update with different state
	orch2 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateCompleted)
	orch2.CreatedTimestamp = createdTime
	orch2.OrchestrationType = "MyWorkflow"
	msg2 := createNatsMsg(t, orch2)
	watcher.onMessage(msg2.Data, msg2)

	entry, err := index.FindByID(ctx, "orch-1")
	require.NoError(t, err)
	assert.Equal(t, "corr-1", entry.CorrelationID)
	assert.Equal(t, model.OrchestrationType("MyWorkflow"), entry.OrchestrationType)
	assert.Equal(t, createdTime.Unix(), entry.CreatedTimestamp.Unix())
	assert.Equal(t, api.OrchestrationStateCompleted, entry.State)
}

func createTestStore(t *testing.T) store.EntityStore[*api.OrchestrationEntry] {
	return memorystore.NewInMemoryEntityStore[*api.OrchestrationEntry]()
}

func createTestWatcher(index store.EntityStore[*api.OrchestrationEntry],
	trxContext store.TransactionContext,
	manager api.ProvisionManager) *OrchestrationIndexWatcher {
	return &OrchestrationIndexWatcher{
		index:            index,
		trxContext:       trxContext,
		monitor:          system.NoopMonitor{},
		provisionManager: manager,
	}
}

func createWatcherOrchestration(id, correlationID string, state api.OrchestrationState) api.Orchestration {
	return api.Orchestration{
		ID:                id,
		CorrelationID:     correlationID,
		State:             state,
		StateTimestamp:    time.Now(),
		CreatedTimestamp:  time.Now().Add(-5 * time.Minute),
		OrchestrationType: "TestType",
		Steps:             []api.OrchestrationStep{},
		ProcessingData:    map[string]any{},
		OutputData:        map[string]any{},
		Completed:         map[string]struct{}{},
	}
}

func createNatsMsg(t *testing.T, orch api.Orchestration) *nats.Msg {
	data, err := json.Marshal(orch)
	require.NoError(t, err)
	return &nats.Msg{Data: data}
}
