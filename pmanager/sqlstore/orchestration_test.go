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

package sqlstore

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/sqlstore"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOrchestrationEntryStore_Creation tests store creation
func TestNewOrchestrationEntryStore_Creation(t *testing.T) {
	estore := newOrchestrationEntryStore()

	require.NotNil(t, estore)
	assert.NotNil(t, estore)
}

// TestNewOrchestrationEntryStore_FindByID_Success tests successful orchestration entry retrieval
func TestNewOrchestrationEntryStore_FindByID_Success(t *testing.T) {
	setupOrchestrationEntryTable(t, testDB)
	defer cleanupOrchestrationEntryTestData(t, testDB)

	entry := &api.OrchestrationEntry{
		ID:                "orch-entry-1",
		Version:           1,
		CorrelationID:     "correlation-abc-123",
		DefinitionID:      "def-1",
		State:             api.OrchestrationStateInitialized,
		StateTimestamp:    time.Now(),
		CreatedTimestamp:  time.Now(),
		OrchestrationType: model.OrchestrationType("provision"),
	}

	_, err := testDB.Exec(
		"INSERT INTO orchestration_entries (id, version, correlation_id, definition_id, state, state_timestamp, created_timestamp, orchestration_type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		entry.ID,
		entry.Version,
		entry.CorrelationID,
		entry.DefinitionID,
		entry.State,
		entry.StateTimestamp,
		entry.CreatedTimestamp,
		entry.OrchestrationType,
	)
	require.NoError(t, err)

	estore := newOrchestrationEntryStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "orch-entry-1")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "orch-entry-1", retrieved.ID)
	assert.Equal(t, int64(1), retrieved.Version)
	assert.Equal(t, "correlation-abc-123", retrieved.CorrelationID)
	assert.Equal(t, api.OrchestrationStateInitialized, retrieved.State)
}

// TestNewOrchestrationEntryStore_FindByID_NotFound tests orchestration entry not found
func TestNewOrchestrationEntryStore_FindByID_NotFound(t *testing.T) {
	setupOrchestrationEntryTable(t, testDB)
	defer cleanupOrchestrationEntryTestData(t, testDB)

	estore := newOrchestrationEntryStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	_, err = estore.FindByID(txCtx, "non-existent")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewOrchestrationEntryStore_Create tests creating a new orchestration entry
func TestNewOrchestrationEntryStore_Create(t *testing.T) {
	setupOrchestrationEntryTable(t, testDB)
	defer cleanupOrchestrationEntryTestData(t, testDB)

	entry := &api.OrchestrationEntry{
		ID:                "orch-entry-new",
		Version:           1,
		CorrelationID:     "correlation-new-123",
		State:             api.OrchestrationStateRunning,
		StateTimestamp:    time.Now(),
		CreatedTimestamp:  time.Now(),
		OrchestrationType: model.OrchestrationType("provision"),
	}

	estore := newOrchestrationEntryStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	created, err := estore.Create(txCtx, entry)
	require.NoError(t, err)
	assert.Equal(t, "orch-entry-new", created.ID)
	assert.Equal(t, int64(1), created.Version)
	assert.Equal(t, "correlation-new-123", created.CorrelationID)
}

// TestNewOrchestrationEntryStore_SearchByStatePredicate tests filtering by state
func TestNewOrchestrationEntryStore_SearchByStatePredicate(t *testing.T) {
	setupOrchestrationEntryTable(t, testDB)
	defer cleanupOrchestrationEntryTestData(t, testDB)

	entries := []*api.OrchestrationEntry{
		{
			ID:                "orch-state-1",
			Version:           1,
			CorrelationID:     "corr-state-001",
			State:             api.OrchestrationStateInitialized,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
		{
			ID:                "orch-state-2",
			Version:           1,
			CorrelationID:     "corr-state-002",
			State:             api.OrchestrationStateRunning,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("deprovision"),
		},
		{
			ID:                "orch-state-3",
			Version:           1,
			CorrelationID:     "corr-state-003",
			State:             api.OrchestrationStateRunning,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
		{
			ID:                "orch-state-4",
			Version:           1,
			CorrelationID:     "corr-state-004",
			State:             api.OrchestrationStateCompleted,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
	}

	for _, entry := range entries {
		_, err := testDB.Exec(
			"INSERT INTO orchestration_entries (id, version, correlation_id, definition_id, state, state_timestamp, created_timestamp, orchestration_type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
			entry.ID,
			entry.Version,
			entry.CorrelationID,
			entry.DefinitionID,
			entry.State,
			entry.StateTimestamp,
			entry.CreatedTimestamp,
			entry.OrchestrationType,
		)
		require.NoError(t, err)
	}

	estore := newOrchestrationEntryStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("state", api.OrchestrationStateRunning)

	count := 0
	for entry, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, entry)
		count++

		assert.Equal(t, api.OrchestrationStateRunning, entry.State)
	}

	assert.Equal(t, 2, count)
}

// TestNewOrchestrationEntryStore_SearchByCorrelationIDPredicate tests filtering by correlationId
func TestNewOrchestrationEntryStore_SearchByCorrelationIDPredicate(t *testing.T) {
	setupOrchestrationEntryTable(t, testDB)
	defer cleanupOrchestrationEntryTestData(t, testDB)

	entries := []*api.OrchestrationEntry{
		{
			ID:                "orch-corr-1",
			Version:           1,
			CorrelationID:     "correlation-001",
			DefinitionID:      "definition-001",
			State:             api.OrchestrationStateInitialized,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
		{
			ID:                "orch-corr-2",
			Version:           1,
			CorrelationID:     "correlation-002",
			DefinitionID:      "definition-002",
			State:             api.OrchestrationStateRunning,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
		{
			ID:                "orch-corr-3",
			Version:           1,
			CorrelationID:     "correlation-001",
			DefinitionID:      "definition-001",
			State:             api.OrchestrationStateCompleted,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("deprovision"),
		},
		{
			ID:                "orch-corr-4",
			Version:           1,
			CorrelationID:     "correlation-003",
			DefinitionID:      "definition-003",
			State:             api.OrchestrationStateRunning,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
	}

	for _, entry := range entries {
		_, err := testDB.Exec(
			"INSERT INTO orchestration_entries (id, version, correlation_id, definition_id, state, state_timestamp, created_timestamp, orchestration_type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
			entry.ID,
			entry.Version,
			entry.CorrelationID,
			entry.DefinitionID,
			entry.State,
			entry.StateTimestamp,
			entry.CreatedTimestamp,
			entry.OrchestrationType,
		)
		require.NoError(t, err)
	}

	estore := newOrchestrationEntryStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("correlationId", "correlation-001")

	count := 0
	for entry, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, entry)
		count++

		assert.Equal(t, "correlation-001", entry.CorrelationID)
	}

	assert.Equal(t, 2, count)
}

// TestNewOrchestrationEntryStore_SearchByStateAndCorrelationIDPredicate tests filtering with multiple conditions
func TestNewOrchestrationEntryStore_SearchByStateAndCorrelationIDPredicate(t *testing.T) {
	setupOrchestrationEntryTable(t, testDB)
	defer cleanupOrchestrationEntryTestData(t, testDB)

	entries := []*api.OrchestrationEntry{
		{
			ID:                "orch-comb-1",
			Version:           1,
			CorrelationID:     "correlation-combined-1",
			DefinitionID:      "definition-combined-1",
			State:             api.OrchestrationStateRunning,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
		{
			ID:                "orch-comb-2",
			Version:           1,
			CorrelationID:     "correlation-combined-1",
			DefinitionID:      "definition-combined-2",
			State:             api.OrchestrationStateCompleted,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
		{
			ID:                "orch-comb-3",
			Version:           1,
			CorrelationID:     "correlation-combined-2",
			DefinitionID:      "definition-combined-2",
			State:             api.OrchestrationStateRunning,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("deprovision"),
		},
		{
			ID:                "orch-comb-4",
			Version:           1,
			CorrelationID:     "correlation-combined-1",
			DefinitionID:      "definition-combined-1",
			State:             api.OrchestrationStateInitialized,
			StateTimestamp:    time.Now(),
			CreatedTimestamp:  time.Now(),
			OrchestrationType: model.OrchestrationType("provision"),
		},
	}

	for _, entry := range entries {
		_, err := testDB.Exec(
			"INSERT INTO orchestration_entries (id, version, correlation_id, definition_id, state, created_timestamp, state_timestamp, orchestration_type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
			entry.ID,
			entry.Version,
			entry.CorrelationID,
			entry.DefinitionID,
			entry.State,
			entry.StateTimestamp,
			entry.CreatedTimestamp,
			entry.OrchestrationType,
		)
		require.NoError(t, err)
	}

	estore := newOrchestrationEntryStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.And(
		query.Eq("correlationId", "correlation-combined-1"),
		query.Eq("state", api.OrchestrationStateRunning),
	)

	count := 0
	for entry, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, entry)
		count++

		assert.Equal(t, "correlation-combined-1", entry.CorrelationID)
		assert.Equal(t, api.OrchestrationStateRunning, entry.State)
	}

	assert.Equal(t, 1, count)
}

func setupOrchestrationEntryTable(t *testing.T, db *sql.DB) {
	err := createOrchestrationEntriesTable(db)
	require.NoError(t, err)
}

func cleanupOrchestrationEntryTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DROP TABLE IF EXISTS orchestration_entries CASCADE")
	require.NoError(t, err)
}
