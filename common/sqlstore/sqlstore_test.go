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
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEntity represents a simple entity with both column and JSONB storage
type testEntity struct {
	ID        string         `json:"id"`
	Value     string         `json:"value"`
	Version   int64          `json:"version"`
	CreatedAt time.Time      `json:"createdAt"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// GetID returns the entity ID
func (e *testEntity) GetID() string {
	return e.ID
}

// GetVersion returns the entity version
func (e *testEntity) GetVersion() int64 {
	return e.Version
}

// IncrementVersion increments the entity version
func (e *testEntity) IncrementVersion() {
	e.Version++
}

// setupEntityTable creates the test table with mixed column and JSONB storage
func setupEntityTable(t *testing.T) {
	_, err := testDB.Exec(`
		DROP TABLE IF EXISTS test_entities CASCADE;
		CREATE TABLE test_entities (
			id TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			version INT DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			metadata JSONB
		);
	`)
	require.NoError(t, err)
}

// recordToEntity converts DatabaseRecord to testEntity
// - id, value, version, created_at come from regular columns
// - metadata comes from JSONB column
func recordToEntity(_ *sql.Tx, record *DatabaseRecord) (*testEntity, error) {
	entity := &testEntity{}

	// Extract all fields - conversion function knows the schema
	if id, ok := record.Values["id"].(string); ok {
		entity.ID = id
	}
	if value, ok := record.Values["value"].(string); ok {
		entity.Value = value
	}
	if version, ok := record.Values["version"].(int64); ok {
		entity.Version = version
	}
	if createdAt, ok := record.Values["created_at"].(time.Time); ok {
		entity.CreatedAt = createdAt
	}

	// Handle JSONB data - stored as []byte in the record
	if metadataBytes, ok := record.Values["metadata"].([]byte); ok && metadataBytes != nil {
		if err := json.Unmarshal(metadataBytes, &entity.Metadata); err != nil {
			return nil, err
		}
	}

	return entity, nil
}

// entityToRecord converts testEntity to DatabaseRecord
// - all fields are stored in Values map
// - conversion function handles serialization
func entityToRecord(entity *testEntity) (*DatabaseRecord, error) {
	record := DatabaseRecord{
		Values: make(map[string]any),
	}

	// Store all fields - let the database driver handle serialization
	record.Values["id"] = entity.ID
	record.Values["value"] = entity.Value
	record.Values["version"] = entity.Version
	record.Values["created_at"] = entity.CreatedAt

	// Serialize metadata to JSONB bytes
	if entity.Metadata != nil {
		metadataBytes, err := json.Marshal(entity.Metadata)
		if err != nil {
			return &record, err
		}
		record.Values["metadata"] = metadataBytes
	}

	return &record, nil
}

// TestNewPostgresEntityStore_Creation tests store creation
func TestNewPostgresEntityStore_Creation(t *testing.T) {
	columnNames := []string{"id", "value", "version", "created_at", "metadata"}

	store := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())

	require.NotNil(t, store)
	assert.NotNil(t, store.matcher)
	assert.NotNil(t, store.recordToEntity)
	assert.NotNil(t, store.entityToRecord)
}

// TestNewPostgresEntityStore_FindByID_Success tests successful entity retrieval
func TestNewPostgresEntityStore_FindByID_Success(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	entity := &testEntity{
		ID:        "entity-1",
		Value:     "Test Entity",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"key": "value"},
	}

	record, err := entityToRecord(entity)
	require.NoError(t, err)

	metadataVal := any(nil)
	if metadataBytes, ok := record.Values["metadata"].([]byte); ok && len(metadataBytes) > 0 {
		metadataVal = metadataBytes
	}

	_, err = testDB.Exec(
		"INSERT INTO test_entities (id, value, version, created_at, metadata) VALUES ($1, $2, $3, $4, $5)",
		record.Values["id"],
		record.Values["value"],
		record.Values["version"],
		record.Values["created_at"],
		metadataVal,
	)
	require.NoError(t, err)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "entity-1")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "entity-1", retrieved.ID)
	assert.Equal(t, "Test Entity", retrieved.Value)
	assert.Equal(t, int64(1), retrieved.Version)
	assert.NotNil(t, retrieved.Metadata)
	assert.Equal(t, "value", retrieved.Metadata["key"])
}

// TestNewPostgresEntityStore_FindByID_NotFound tests entity not found
func TestNewPostgresEntityStore_FindByID_NotFound(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	_, err = estore.FindByID(txCtx, "non-existent")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewPostgresEntityStore_Exists tests checking entity existence
func TestNewPostgresEntityStore_Exists(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	entity := &testEntity{
		ID:        "exists-entity",
		Value:     "Exists",
		Version:   1,
		CreatedAt: time.Now(),
	}

	record, err := entityToRecord(entity)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO test_entities (id, value, version, created_at) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["value"],
		record.Values["version"],
		record.Values["created_at"],
	)
	require.NoError(t, err)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	exists, err := estore.Exists(txCtx, "exists-entity")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = estore.Exists(txCtx, "non-existent")
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestNewPostgresEntityStore_Create tests creating a new entity
func TestNewPostgresEntityStore_Create(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	entity := &testEntity{
		ID:        "new-entity",
		Value:     "New Entity",
		Version:   1,
		CreatedAt: time.Now(),
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	created, err := estore.Create(txCtx, entity)
	require.NoError(t, err)
	assert.Equal(t, "new-entity", created.ID)
	assert.Equal(t, "New Entity", created.Value)
}

// TestNewPostgresEntityStore_Update tests updating an entity
func TestNewPostgresEntityStore_Update(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	entity := &testEntity{
		ID:        "update-entity",
		Value:     "Original",
		Version:   1,
		CreatedAt: time.Now(),
	}

	record, err := entityToRecord(entity)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO test_entities (id, value, version, created_at) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["value"],
		record.Values["version"],
		record.Values["created_at"],
	)
	require.NoError(t, err)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	entity.Value = "Updated"
	entity.IncrementVersion()
	entity.Metadata = map[string]any{"updated": true}
	err = estore.Update(txCtx, entity)
	require.NoError(t, err)

	updated, err := estore.FindByID(txCtx, entity.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Value)
	assert.Equal(t, true, updated.Metadata["updated"])

}

// TestNewPostgresEntityStore_Delete tests deleting an entity
func TestNewPostgresEntityStore_Delete(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	entity := &testEntity{
		ID:        "delete-entity",
		Value:     "To Delete",
		Version:   1,
		CreatedAt: time.Now(),
	}

	record, err := entityToRecord(entity)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO test_entities (id, value, version, created_at) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["value"],
		record.Values["version"],
		record.Values["created_at"],
	)
	require.NoError(t, err)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	err = estore.Delete(txCtx, "delete-entity")
	require.NoError(t, err)

	_, err = estore.FindByID(txCtx, "delete-entity")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewPostgresEntityStore_GetAll tests retrieving all entities
func TestNewPostgresEntityStore_GetAll(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	for i := 1; i <= 3; i++ {
		entity := &testEntity{
			ID:        "entity-" + strconv.Itoa(i),
			Value:     "Entity " + strconv.Itoa(i),
			Version:   1,
			CreatedAt: time.Now(),
		}
		record, err := entityToRecord(entity)
		require.NoError(t, err)
		_, err = testDB.Exec(
			"INSERT INTO test_entities (id, value, version, created_at) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["value"],
			record.Values["version"],
			record.Values["created_at"],
		)
		require.NoError(t, err)
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	count := 0
	for entity, err := range estore.GetAll(txCtx) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		count++
	}
	assert.Equal(t, 3, count)
}

// TestNewPostgresEntityStore_GetAllCount tests counting all entities
func TestNewPostgresEntityStore_GetAllCount(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	for i := 1; i <= 5; i++ {
		entity := &testEntity{
			ID:        "entity-" + strconv.Itoa(i),
			Value:     "Entity " + strconv.Itoa(i),
			Version:   1,
			CreatedAt: time.Now(),
		}
		record, err := entityToRecord(entity)
		require.NoError(t, err)
		_, err = testDB.Exec(
			"INSERT INTO test_entities (id, value, version, created_at) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["value"],
			record.Values["version"],
			record.Values["created_at"],
		)
		require.NoError(t, err)
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	count, err := estore.GetAllCount(txCtx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

// TestNewPostgresEntityStore_GetAllPaginated tests paginated retrieval
func TestNewPostgresEntityStore_GetAllPaginated(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	for i := 1; i <= 10; i++ {
		entity := &testEntity{
			ID:        "entity-" + strconv.Itoa(i),
			Value:     "Entity " + strconv.Itoa(i),
			Version:   1,
			CreatedAt: time.Now(),
		}
		record, err := entityToRecord(entity)
		require.NoError(t, err)
		_, err = testDB.Exec(
			"INSERT INTO test_entities (id, value, version, created_at) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["value"],
			record.Values["version"],
			record.Values["created_at"],
		)
		require.NoError(t, err)
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	opts := store.PaginationOptions{Offset: 0, Limit: 3}
	count := 0
	for entity, err := range estore.GetAllPaginated(txCtx, opts) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		count++
	}
	assert.Equal(t, 3, count)
}

// TestNewPostgresEntityStore_WithMetadata tests entity with metadata in JSONB
func TestNewPostgresEntityStore_WithMetadata(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	entity := &testEntity{
		ID:        "entity-with-metadata",
		Value:     "Complex Entity",
		Version:   2,
		CreatedAt: time.Now(),
		Metadata: map[string]any{
			"owner":  "test-user",
			"tags":   []string{"important", "test"},
			"nested": map[string]any{"deep": "value"},
			"count":  42,
		},
	}

	record, err := entityToRecord(entity)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO test_entities (id, value, version, created_at, metadata) VALUES ($1, $2, $3, $4, $5)",
		record.Values["id"],
		record.Values["value"],
		record.Values["version"],
		record.Values["created_at"],
		record.Values["metadata"],
	)
	require.NoError(t, err)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "entity-with-metadata")
	require.NoError(t, err)
	assert.Equal(t, int64(2), retrieved.Version)
	assert.Equal(t, "Complex Entity", retrieved.Value)
	assert.NotNil(t, retrieved.Metadata)
	assert.Equal(t, "test-user", retrieved.Metadata["owner"])
	assert.Equal(t, float64(42), retrieved.Metadata["count"])
}

// TestNewPostgresEntityStore_VersionIncrement tests version incrementing
func TestNewPostgresEntityStore_VersionIncrement(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	entity := &testEntity{
		ID:        "entity-version",
		Value:     "Version Test",
		Version:   1,
		CreatedAt: time.Now(),
	}

	record, err := entityToRecord(entity)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO test_entities (id, value, version, created_at) VALUES ($1, $2, $3, $4)",
		record.Values["id"],
		record.Values["value"],
		record.Values["version"],
		record.Values["created_at"],
	)
	require.NoError(t, err)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "entity-version")
	require.NoError(t, err)
	assert.Equal(t, int64(1), retrieved.Version)

	retrieved.IncrementVersion()
	assert.Equal(t, int64(2), retrieved.Version)
}

func createBuilder() *JSONBSQLBuilder {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})
	return &builder
}
