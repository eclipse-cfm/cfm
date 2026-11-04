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
	"errors"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	aMocks "github.com/eclipse-cfm/cfm/pmanager/api/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// FindByID returns error - verify Nak is called exactly once
func TestOnMessage_FindByIDError_NakCalledOnce(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	expectedErr := errors.New("database connection failed")

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(nil, expectedErr).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 1, msg.NakCalls, "Nak should be called exactly once when FindByID returns error")
	assert.Equal(t, 0, msg.AckCalls, "Ack should not be called when Nak is called")
	mockStore.AssertExpectations(t)
}

// Create returns error - verify Nak is called once, Ack not called
func TestOnMessage_CreateError_NakCalledNotAck(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	expectedErr := errors.New("failed to write to database")

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(nil, types.ErrNotFound).
		Once()

	mockStore.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(entry *api.OrchestrationEntry) bool {
			return entry.ID == "orch-1"
		})).
		Return(nil, expectedErr).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 1, msg.NakCalls, "Nak should be called exactly once when Create returns error")
	assert.Equal(t, 0, msg.AckCalls, "Ack should not be called on Create error")
	mockStore.AssertExpectations(t)
}

// Update returns error - verify Nak is called once, Ack not called
func TestOnMessage_UpdateError_NakCalledNotAck(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	existingEntry := &api.OrchestrationEntry{
		ID:                "orch-1",
		CorrelationID:     "corr-1",
		State:             api.OrchestrationStateRunning,
		StateTimestamp:    time.Now(),
		CreatedTimestamp:  time.Now(),
		OrchestrationType: "TestType",
	}

	expectedErr := errors.New("update failed - constraint violation")

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(existingEntry, nil).
		Once()

	mockStore.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(entry *api.OrchestrationEntry) bool {
			return entry.ID == "orch-1"
		})).
		Return(expectedErr).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateCompleted)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 1, msg.NakCalls, "Nak should be called exactly once when Update returns error")
	assert.Equal(t, 0, msg.AckCalls, "Ack should not be called on Update error")
	mockStore.AssertExpectations(t)
}

// FindByID unexpected error - verify Nak is called, no further operations
func TestOnMessage_FindByIDUnexpectedError_NakCalledImmediately(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	unexpectedErr := errors.New("data corruption detected")

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(nil, unexpectedErr).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 1, msg.NakCalls, "Nak should be called for FindByID error")
	assert.Equal(t, 0, msg.AckCalls, "Ack should not be called")
	mockStore.AssertExpectations(t)
}

// Multiple sequential index errors - each results in single Nak
func TestOnMessage_SequentialErrors_EachNakOnce(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	dbError := errors.New("database unavailable")

	// First message - FindByID fails
	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(nil, dbError).
		Once()

	orch1 := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	data1, _ := json.Marshal(orch1)
	msg1 := NewMockMessage(data1)

	watcher.onMessage(data1, msg1)
	assert.Equal(t, 1, msg1.NakCalls, "First message should have 1 Nak")
	assert.Equal(t, 0, msg1.AckCalls, "First message should have 0 Ack")

	// Second message - Create fails
	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-2").
		Return(nil, types.ErrNotFound).
		Once()

	mockStore.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(entry *api.OrchestrationEntry) bool {
			return entry.ID == "orch-2"
		})).
		Return(nil, dbError).
		Once()

	orch2 := createWatcherOrchestration("orch-2", "corr-2", api.OrchestrationStateRunning)
	data2, _ := json.Marshal(orch2)
	msg2 := NewMockMessage(data2)

	watcher.onMessage(data2, msg2)
	assert.Equal(t, 1, msg2.NakCalls, "Second message should have 1 Nak")
	assert.Equal(t, 0, msg2.AckCalls, "Second message should have 0 Ack")

	mockStore.AssertExpectations(t)
}

// Transient Create error - verify Nak for retry
func TestOnMessage_CreateTransientError_NakForRetry(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	transientErr := errors.New("temporary lock timeout")

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(nil, types.ErrNotFound).
		Once()

	mockStore.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(entry *api.OrchestrationEntry) bool {
			return entry.ID == "orch-1"
		})).
		Return(nil, transientErr).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 1, msg.NakCalls, "Nak should be called once for transient error")
	assert.Equal(t, 0, msg.AckCalls, "Ack should not be called")
	mockStore.AssertExpectations(t)
}

// Update with state conflict - verify Nak for retry
func TestOnMessage_UpdateStateConflict_NakForRetry(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	existingEntry := &api.OrchestrationEntry{
		ID:                "orch-1",
		CorrelationID:     "corr-1",
		State:             api.OrchestrationStateRunning,
		StateTimestamp:    time.Now(),
		CreatedTimestamp:  time.Now(),
		OrchestrationType: "TestType",
	}

	stateConflictErr := errors.New("version mismatch")

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(existingEntry, nil).
		Once()

	mockStore.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(entry *api.OrchestrationEntry) bool {
			return entry.ID == "orch-1"
		})).
		Return(stateConflictErr).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateCompleted)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 1, msg.NakCalls, "Nak should be called once for state conflict")
	assert.Equal(t, 0, msg.AckCalls, "Ack should not be called")
	mockStore.AssertExpectations(t)
}

// No Nak when successful create
func TestOnMessage_SuccessfulCreate_AckCalledNotNak(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(nil, types.ErrNotFound).
		Once()

	mockStore.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(entry *api.OrchestrationEntry) bool {
			return entry.ID == "orch-1"
		})).
		Return(&api.OrchestrationEntry{}, nil).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 0, msg.NakCalls, "Nak should not be called on successful create")
	assert.Equal(t, 1, msg.AckCalls, "Ack should be called once on successful create")
	mockStore.AssertExpectations(t)
}

// No Nak when successful update
func TestOnMessage_SuccessfulUpdate_AckCalledNotNak(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	existingEntry := &api.OrchestrationEntry{
		ID:                "orch-1",
		CorrelationID:     "corr-1",
		State:             api.OrchestrationStateRunning,
		StateTimestamp:    time.Now(),
		CreatedTimestamp:  time.Now(),
		OrchestrationType: "TestType",
	}

	mockStore.EXPECT().
		FindByID(mock.Anything, "orch-1").
		Return(existingEntry, nil).
		Once()

	mockStore.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(entry *api.OrchestrationEntry) bool {
			return entry.ID == "orch-1"
		})).
		Return(nil).
		Once()

	orch := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateCompleted)
	data, _ := json.Marshal(orch)
	msg := NewMockMessage(data)

	watcher.onMessage(data, msg)

	assert.Equal(t, 0, msg.NakCalls, "Nak should not be called on successful update")
	assert.Equal(t, 1, msg.AckCalls, "Ack should be called once on successful update")
	mockStore.AssertExpectations(t)
}

// Malformed JSON - verify Ack is called (not Nak)
func TestOnMessage_MalformedJSON_AckCalled(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := &store.NoOpTransactionContext{}
	watcher := createTestWatcher(mockStore, trxContext, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	msg := NewMockMessage([]byte("invalid json"))

	watcher.onMessage([]byte("invalid json"), msg)

	assert.Equal(t, 0, msg.NakCalls, "Nak should not be called for malformed JSON")
	assert.Equal(t, 1, msg.AckCalls, "Ack should be called for malformed JSON")
	mockStore.AssertExpectations(t)
}

// Compensation start fails - verify the message is rejected with a delay so that the failure is retried
func TestOnMessage_CompensationStartError_NakWithDelay(t *testing.T) {
	index := createTestStore(t)
	pManager := &aMocks.MockProvisionManager{}
	pManager.On("Start", mock.Anything, mock.Anything).Return(nil, errors.New("kv unavailable"))
	definitionManager := &aMocks.MockDefinitionManager{}
	definitionManager.On("QueryOrchestrationDefinitions", mock.Anything, mock.Anything).Return([]api.OrchestrationDefinition{{}}, nil)
	watcher := createTestWatcher(index, &store.NoOpTransactionContext{}, pManager, definitionManager)

	running := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	_, err := index.Create(t.Context(), convertToEntry(running))
	assert.NoError(t, err)

	errored := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateErrored)
	errored.StateTimestamp = running.StateTimestamp.Add(time.Second)
	msg := createNatsMsg(t, errored)

	watcher.onMessage(msg.Data, msg)

	assert.Equal(t, 0, msg.AckCalls)
	assert.Equal(t, []time.Duration{watcherRedeliveryDelay}, msg.NakDelays)
	pManager.AssertExpectations(t)
}

// Commit fails after the index was written - verify the message is not acknowledged
func TestOnMessage_CommitError_NakNotAck(t *testing.T) {
	mockStore := mocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	watcher := createTestWatcher(mockStore, failingCommitTransactionContext{}, &aMocks.MockProvisionManager{}, &aMocks.MockDefinitionManager{})

	mockStore.EXPECT().FindByID(mock.Anything, "orch-1").Return(nil, types.ErrNotFound).Once()
	mockStore.EXPECT().Create(mock.Anything, mock.Anything).Return(&api.OrchestrationEntry{}, nil).Once()

	msg := createNatsMsg(t, createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning))

	watcher.onMessage(msg.Data, msg)

	assert.Equal(t, 0, msg.AckCalls, "Ack must not be sent before the index transaction has committed")
	assert.Equal(t, 1, msg.NakCalls)
}

// The same failure handled twice starts the same compensation, so the provision manager can de-duplicate it
func TestOnMessage_RedeliveredFailure_SameCompensationID(t *testing.T) {
	var manifestIDs []string
	pManager := &aMocks.MockProvisionManager{}
	pManager.On("Start", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		manifestIDs = append(manifestIDs, args.Get(1).(*model.OrchestrationManifest).ID)
	}).Return(&api.Orchestration{}, nil)
	definitionManager := &aMocks.MockDefinitionManager{}
	definitionManager.On("QueryOrchestrationDefinitions", mock.Anything, mock.Anything).Return([]api.OrchestrationDefinition{{}}, nil)

	running := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateRunning)
	errored := createWatcherOrchestration("orch-1", "corr-1", api.OrchestrationStateErrored)
	errored.StateTimestamp = running.StateTimestamp.Add(time.Second)

	// two replicas, each with an index that has not seen the failure yet
	for range 2 {
		index := createTestStore(t)
		_, err := index.Create(t.Context(), convertToEntry(running))
		assert.NoError(t, err)
		watcher := createTestWatcher(index, &store.NoOpTransactionContext{}, pManager, definitionManager)
		msg := createNatsMsg(t, errored)
		watcher.onMessage(msg.Data, msg)
		assert.Equal(t, 1, msg.AckCalls)
	}

	assert.Equal(t, []string{"orch-1-compensation", "orch-1-compensation"}, manifestIDs)
}

// failingCommitTransactionContext runs the callback and then fails, as a commit error would.
type failingCommitTransactionContext struct{}

func (failingCommitTransactionContext) Execute(ctx context.Context, callback func(ctx context.Context) error) error {
	if err := callback(ctx); err != nil {
		return err
	}
	return errors.New("commit failed")
}

// MockMessage implements MessageAck interface for testing Nak/Ack calls
type MockMessage struct {
	Data      []byte
	NakCalls  int
	AckCalls  int
	NakDelays []time.Duration
}

func NewMockMessage(data []byte) *MockMessage {
	return &MockMessage{Data: data}
}

func (m *MockMessage) NakWithDelay(delay time.Duration) error {
	m.NakCalls++
	m.NakDelays = append(m.NakDelays, delay)
	return nil
}

func (m *MockMessage) Ack() error {
	m.AckCalls++
	return nil
}
