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
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCellStore_Creation tests store creation
func TestNewCellStore_Creation(t *testing.T) {
	cstore := newCellStore()

	require.NotNil(t, cstore)
	assert.NotNil(t, cstore)
}

// TestNewCellStore_FindByID_Success tests successful cell retrieval
func TestNewCellStore_FindByID_Success(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-1",
				Version: 1,
			},
			State:          api.DeploymentStateActive,
			StateTimestamp: time.Now(),
		},
		Properties: map[string]any{
			"metadata": map[string]any{"key": "value"},
		},
	}

	record, err := cellEntityToRecord(cell)
	require.NoError(t, err)

	propertiesVal := any(nil)
	if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
		propertiesVal = propertiesBytes
	}

	_, err = testDB.Exec(
		"INSERT INTO cells (id, version, state, state_timestamp, properties) VALUES ($1, $2, $3, $4, $5)",
		record.Values["id"],
		record.Values["version"],
		record.Values["state"],
		record.Values["state_timestamp"],
		propertiesVal,
	)
	require.NoError(t, err)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "cell-1")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "cell-1", retrieved.ID)
	assert.Equal(t, api.DeploymentStateActive, retrieved.State)
	assert.Equal(t, int64(1), retrieved.Version)
}

// TestNewCellStore_FindByID_NotFound tests cell not found
func TestNewCellStore_FindByID_NotFound(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	_, err = estore.FindByID(txCtx, "non-existent")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewCellStore_Exists tests checking cell existence
func TestNewCellStore_Exists(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "exists-cell",
				Version: 1,
			},
			State:          api.DeploymentStatePending,
			StateTimestamp: time.Now(),
		},
		Properties: nil,
	}

	record, err := cellEntityToRecord(cell)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO cells (id, version, state, state_timestamp) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["version"],
		record.Values["state"],
		record.Values["state_timestamp"],
	)
	require.NoError(t, err)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	exists, err := estore.Exists(txCtx, "exists-cell")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = estore.Exists(txCtx, "non-existent")
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestNewCellStore_Create tests creating a new cell
func TestNewCellStore_Create(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "new-cell",
				Version: 1,
			},
			State:          api.DeploymentStatePending,
			StateTimestamp: time.Now(),
		},
		ExternalID: "external-id",
		Properties: map[string]any{
			"metadata": map[string]any{"owner": "test"},
		},
	}

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	created, err := estore.Create(txCtx, cell)
	require.NoError(t, err)
	assert.Equal(t, "new-cell", created.ID)
	assert.Equal(t, "external-id", created.ExternalID)
	assert.Equal(t, api.DeploymentStatePending, created.State)
}

// TestNewCellStore_Update tests updating a cell
func TestNewCellStore_Update(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "update-cell",
				Version: 1,
			},
			State:          api.DeploymentStatePending,
			StateTimestamp: time.Now(),
		},
		Properties: nil,
	}

	record, err := cellEntityToRecord(cell)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO cells (id, version, state, state_timestamp) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["version"],
		record.Values["state"],
		record.Values["state_timestamp"],
	)
	require.NoError(t, err)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	cell.State = api.DeploymentStateActive
	cell.StateTimestamp = time.Now()
	cell.IncrementVersion()
	err = estore.Update(txCtx, cell)
	require.NoError(t, err)

	updated, err := estore.FindByID(txCtx, cell.ID)
	require.NoError(t, err)
	assert.Equal(t, api.DeploymentStateActive, updated.State)
	assert.Equal(t, int64(2), updated.Version)
}

// TestNewCellStore_Delete tests deleting a cell
func TestNewCellStore_Delete(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "delete-cell",
				Version: 1,
			},
			State:          api.DeploymentStateDisposed,
			StateTimestamp: time.Now(),
		},
		Properties: nil,
	}

	record, err := cellEntityToRecord(cell)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO cells (id, version, state, state_timestamp) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["version"],
		record.Values["state"],
		record.Values["state_timestamp"],
	)
	require.NoError(t, err)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	err = estore.Delete(txCtx, "delete-cell")
	require.NoError(t, err)

	_, err = estore.FindByID(txCtx, "delete-cell")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewCellStore_GetAll tests retrieving all cells
func TestNewCellStore_GetAll(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	states := []api.DeploymentState{
		api.DeploymentStatePending,
		api.DeploymentStateActive,
		api.DeploymentStateDisposed,
	}

	for i := 1; i <= 3; i++ {
		cell := &api.Cell{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      "cell-" + strconv.Itoa(i),
					Version: 1,
				},
				State:          states[i-1],
				StateTimestamp: time.Now(),
			},
			Properties: nil,
		}

		record, err := cellEntityToRecord(cell)
		require.NoError(t, err)
		_, err = testDB.Exec(
			"INSERT INTO cells (id, version, state, state_timestamp) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["version"],
			record.Values["state"],
			record.Values["state_timestamp"],
		)
		require.NoError(t, err)
	}

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	count := 0
	for cell, err := range estore.GetAll(txCtx) {
		require.NoError(t, err)
		require.NotNil(t, cell)
		count++
	}
	assert.Equal(t, 3, count)
}

// TestNewCellStore_Create_ExternalID_Unique tests that externalID uniqueness is enforced
func TestNewCellStore_Create_ExternalID_Unique(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	// Create first cell with an externalID
	firstCell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-unique-1",
				Version: 1,
			},
			State:          api.DeploymentStateActive,
			StateTimestamp: time.Now(),
		},
		ExternalID: "unique-external-id",
		Properties: map[string]any{
			"owner": "test",
		},
	}

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Create the first cell successfully
	created, err := estore.Create(txCtx, firstCell)
	require.NoError(t, err)
	assert.Equal(t, "cell-unique-1", created.ID)
	assert.Equal(t, "unique-external-id", created.ExternalID)

	// Attempt to create a second cell with the same externalID
	secondCell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-unique-2",
				Version: 1,
			},
			State:          api.DeploymentStateActive,
			StateTimestamp: time.Now(),
		},
		ExternalID: "unique-external-id", // Same externalID as first cell
		Properties: map[string]any{
			"owner": "test",
		},
	}

	// This should fail due to unique constraint violation
	_, err = estore.Create(txCtx, secondCell)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique")
}

// TestNewCellStore_Create_ExternalID_Empty_Multiple tests that cells with empty externalID can be stored multiple times
func TestNewCellStore_Create_ExternalID_Empty_Multiple(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Create first cell with empty externalID
	firstCell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-empty-ext-1",
				Version: 1,
			},
			State:          api.DeploymentStateActive,
			StateTimestamp: time.Now(),
		},
		ExternalID: "",
		Properties: map[string]any{
			"owner": "test",
		},
	}

	created, err := estore.Create(txCtx, firstCell)
	require.NoError(t, err)
	assert.Equal(t, "cell-empty-ext-1", created.ID)
	assert.Equal(t, "", created.ExternalID)

	// Create second cell with empty externalID
	secondCell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-empty-ext-2",
				Version: 1,
			},
			State:          api.DeploymentStateActive,
			StateTimestamp: time.Now(),
		},
		ExternalID: "", // Empty externalID like the first cell
		Properties: map[string]any{
			"owner": "test",
		},
	}

	// This should succeed - empty externalIDs should not violate unique constraint
	created2, err := estore.Create(txCtx, secondCell)
	require.NoError(t, err)
	assert.Equal(t, "cell-empty-ext-2", created2.ID)
	assert.Equal(t, "", created2.ExternalID)

	// Create a third cell with empty externalID to further verify
	thirdCell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-empty-ext-3",
				Version: 1,
			},
			State:          api.DeploymentStatePending,
			StateTimestamp: time.Now(),
		},
		ExternalID: "",
		Properties: nil,
	}

	created3, err := estore.Create(txCtx, thirdCell)
	require.NoError(t, err)
	assert.Equal(t, "cell-empty-ext-3", created3.ID)
	assert.Equal(t, "", created3.ExternalID)
}

// TestNewCellStore_GetAllCount tests counting all cells
func TestNewCellStore_GetAllCount(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	for i := 1; i <= 5; i++ {
		cell := &api.Cell{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      "cell-" + strconv.Itoa(i),
					Version: 1,
				},
				State:          api.DeploymentStateActive,
				StateTimestamp: time.Now(),
			},
			Properties: nil,
		}

		record, err := cellEntityToRecord(cell)
		require.NoError(t, err)
		_, err = testDB.Exec(
			"INSERT INTO cells (id, version, state, state_timestamp) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["version"],
			record.Values["state"],
			record.Values["state_timestamp"],
		)
		require.NoError(t, err)
	}

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	count, err := estore.GetAllCount(txCtx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

// TestNewCellStore_GetAllPaginated tests paginated cell retrieval
func TestNewCellStore_GetAllPaginated(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	for i := 1; i <= 10; i++ {
		cell := &api.Cell{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      "cell-" + strconv.Itoa(i),
					Version: 1,
				},
				State:          api.DeploymentStateActive,
				StateTimestamp: time.Now(),
			},
			Properties: nil,
		}

		record, err := cellEntityToRecord(cell)
		require.NoError(t, err)
		_, err = testDB.Exec(
			"INSERT INTO cells (id, version, state, state_timestamp) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["version"],
			record.Values["state"],
			record.Values["state_timestamp"],
		)
		require.NoError(t, err)
	}

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	opts := store.PaginationOptions{Offset: 0, Limit: 3}
	count := 0
	for cell, err := range estore.GetAllPaginated(txCtx, opts) {
		require.NoError(t, err)
		require.NotNil(t, cell)
		count++
	}
	assert.Equal(t, 3, count)
}

// TestNewCellStore_WithProperties tests cell with properties in JSONB
func TestNewCellStore_WithProperties(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-with-properties",
				Version: 2,
			},
			State:          api.DeploymentStateActive,
			StateTimestamp: time.Now(),
		},
		Properties: map[string]any{
			"metadata": map[string]any{
				"owner":  "test-user",
				"tags":   []string{"important", "test"},
				"nested": map[string]any{"deep": "value"},
				"count":  42,
			},
		},
	}

	record, err := cellEntityToRecord(cell)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO cells (id, version, state, state_timestamp, properties) VALUES ($1, $2, $3, $4, $5)",
		record.Values["id"],
		record.Values["version"],
		record.Values["state"],
		record.Values["state_timestamp"],
		record.Values["properties"],
	)
	require.NoError(t, err)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "cell-with-properties")
	require.NoError(t, err)
	assert.Equal(t, int64(2), retrieved.Version)
	assert.Equal(t, api.DeploymentStateActive, retrieved.State)
	assert.NotNil(t, retrieved.Properties)
	assert.NotNil(t, retrieved.Properties["metadata"])
}

// TestNewCellStore_StateTransitions tests cell state transitions
func TestNewCellStore_StateTransitions(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cell := &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-state-transitions",
				Version: 1,
			},
			State:          api.DeploymentStatePending,
			StateTimestamp: time.Now(),
		},
		Properties: nil,
	}

	record, err := cellEntityToRecord(cell)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO cells (id, version, state, state_timestamp) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["version"],
		record.Values["state"],
		record.Values["state_timestamp"],
	)
	require.NoError(t, err)

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Retrieve and update to Running state
	retrieved, err := estore.FindByID(txCtx, "cell-state-transitions")
	require.NoError(t, err)
	assert.Equal(t, api.DeploymentStatePending, retrieved.State)

	retrieved.State = api.DeploymentStateActive
	retrieved.StateTimestamp = time.Now()
	retrieved.IncrementVersion()
	err = estore.Update(txCtx, retrieved)
	require.NoError(t, err)

	// Verify state change
	updated, err := estore.FindByID(txCtx, "cell-state-transitions")
	require.NoError(t, err)
	assert.Equal(t, api.DeploymentStateActive, updated.State)
	assert.Equal(t, int64(2), updated.Version)
}

// TestNewCellStore_SearchByPropertiesPredicate tests searching cells by a predicate on properties key
func TestNewCellStore_SearchByPropertiesPredicate(t *testing.T) {
	setupCellTable(t, testDB)
	defer cleanupTestData(t, testDB)

	cells := []*api.Cell{
		{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      "cell-prod-1",
					Version: 1,
				},
				State:          api.DeploymentStateActive,
				StateTimestamp: time.Now(),
			},
			Properties: map[string]any{
				"environment": "production",
				"region":      "us-east-1",
				"tier":        "premium",
			},
		},
		{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      "cell-prod-2",
					Version: 1,
				},
				State:          api.DeploymentStateActive,
				StateTimestamp: time.Now(),
			},
			Properties: map[string]any{
				"environment": "production",
				"region":      "us-west-2",
				"tier":        "standard",
			},
		},
		{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      "cell-staging-1",
					Version: 1,
				},
				State:          api.DeploymentStatePending,
				StateTimestamp: time.Now(),
			},
			Properties: map[string]any{
				"environment": "staging",
				"region":      "us-east-1",
				"tier":        "standard",
			},
		},
	}

	// Insert test cells
	for _, cell := range cells {
		record, err := cellEntityToRecord(cell)
		require.NoError(t, err)

		propertiesVal := any(nil)
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO cells (id, version, state, state_timestamp, properties) VALUES ($1, $2, $3, $4, $5)",
			record.Values["id"],
			record.Values["version"],
			record.Values["state"],
			record.Values["state_timestamp"],
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newCellStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("properties.environment", "production")

	count := 0
	for cell, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, cell)
		count++

		// Verify the retrieved cell has the correct property value
		if env, ok := cell.Properties["environment"].(string); ok {
			assert.Equal(t, "production", env)
		}
	}

	assert.Equal(t, 2, count)
}

// cleanupTestData removes test data from the cells table
func cleanupTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", cfmCellsTable))
	require.NoError(t, err)
}

// setupCellTable creates the cells table for testing
func setupCellTable(t *testing.T, db *sql.DB) {
	err := createCellsTable(db)
	require.NoError(t, err)
}
