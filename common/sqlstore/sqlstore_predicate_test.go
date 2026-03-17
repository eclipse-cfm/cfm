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
	"errors"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresEntityStore_FindByPredicate_MetadataProperty tests FindByPredicate with metadata JSONB property search
func TestPostgresEntityStore_FindByPredicate_MetadataProperty(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "entity-meta-1",
		Value:     "Entity with environment prod",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "prod", "region": "us-east"},
	}

	entity2 := &testEntity{
		ID:        "entity-meta-2",
		Value:     "Entity with environment staging",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "staging", "region": "eu-west"},
	}

	entity3 := &testEntity{
		ID:        "entity-meta-3",
		Value:     "Entity with environment prod",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "prod", "region": "us-west"},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("metadata.environment", "prod")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Equal(t, "prod", entity.Metadata["environment"])
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_SimpleColumn tests FindByPredicate with simple column search
func TestPostgresEntityStore_FindByPredicate_SimpleColumn(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "simple-1",
		Value:     "Test Value A",
		Version:   1,
		CreatedAt: time.Now(),
	}

	entity2 := &testEntity{
		ID:        "simple-2",
		Value:     "Test Value B",
		Version:   1,
		CreatedAt: time.Now(),
	}

	entity3 := &testEntity{
		ID:        "simple-3",
		Value:     "Test Value A",
		Version:   2,
		CreatedAt: time.Now(),
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	// Test predicate on simple column
	predicate := query.Eq("value", "Test Value A")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Equal(t, "Test Value A", entity.Value)
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicatePaginated tests FindByPredicatePaginated with limit and offset
func TestPostgresEntityStore_FindByPredicatePaginated(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert 5 test entities
	for i := 1; i <= 5; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "paged-" + string(rune(i)),
			Value:     "Paginated Entity",
			Version:   1,
			CreatedAt: time.Now(),
		})
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("value", "Paginated Entity")

	// Test first page
	opts := store.PaginationOptions{Limit: 2, Offset: 0}
	count := 0
	for entity, err := range estore.FindByPredicatePaginated(txCtx, predicate, opts) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		count++
	}
	assert.Equal(t, 2, count)

	// Test second page
	opts = store.PaginationOptions{Limit: 2, Offset: 2}
	count = 0
	for entity, err := range estore.FindByPredicatePaginated(txCtx, predicate, opts) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		count++
	}
	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindFirstByPredicate tests FindFirstByPredicate returns only first entity
func TestPostgresEntityStore_FindFirstByPredicate(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert multiple test entities with same value
	for i := 1; i <= 3; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "first-" + string(rune(i)),
			Value:     "First Test",
			Version:   1,
			CreatedAt: time.Now(),
		})
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("value", "First Test")

	entity, err := estore.FindFirstByPredicate(txCtx, predicate)
	require.NoError(t, err)
	require.NotNil(t, entity)
	assert.Equal(t, "First Test", entity.Value)
}

// TestPostgresEntityStore_FindFirstByPredicate_NotFound tests FindFirstByPredicate returns error when not found
func TestPostgresEntityStore_FindFirstByPredicate_NotFound(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("value", "Non-Existent")

	_, err = estore.FindFirstByPredicate(txCtx, predicate)
	require.Error(t, err)
	assert.True(t, errors.Is(err, types.ErrNotFound), "Error should be ErrNotFound")
}

// TestPostgresEntityStore_CountByPredicate tests CountByPredicate
func TestPostgresEntityStore_CountByPredicate(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	for i := 1; i <= 3; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "count-" + string(rune(i)),
			Value:     "Count Test",
			Version:   1,
			CreatedAt: time.Now(),
		})
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("value", "Count Test")

	count, err := estore.CountByPredicate(txCtx, predicate)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestPostgresEntityStore_CountByPredicate_NoMatches tests CountByPredicate returns 0 when no matches
func TestPostgresEntityStore_CountByPredicate_NoMatches(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("value", "Non-Existent")

	count, err := estore.CountByPredicate(txCtx, predicate)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// TestPostgresEntityStore_CountByPredicate_NilPredicate tests CountByPredicate with nil predicate returns all count
func TestPostgresEntityStore_CountByPredicate_NilPredicate(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	for i := 1; i <= 2; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "count-all-" + string(rune(i)),
			Value:     "Various",
			Version:   1,
			CreatedAt: time.Now(),
		})
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	count, err := estore.CountByPredicate(txCtx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// TestPostgresEntityStore_DeleteByPredicate tests DeleteByPredicate removes matching entities
func TestPostgresEntityStore_DeleteByPredicate(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	toDelete := []*testEntity{
		{ID: "del-1", Value: "Delete Me", Version: 1, CreatedAt: time.Now()},
		{ID: "del-2", Value: "Delete Me", Version: 1, CreatedAt: time.Now()},
	}
	toKeep := []*testEntity{
		{ID: "keep-1", Value: "Keep Me", Version: 1, CreatedAt: time.Now()},
	}

	for _, entity := range append(toDelete, toKeep...) {
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("value", "Delete Me")

	err = estore.DeleteByPredicate(txCtx, predicate)
	require.NoError(t, err)

	// Verify deletions
	var count int64
	row := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_entities WHERE value = 'Delete Me'")
	err = row.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Verify others still exist
	row = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_entities WHERE value = 'Keep Me'")
	err = row.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestPostgresEntityStore_DeleteByPredicate_NotFound tests DeleteByPredicate returns error when no matches
func TestPostgresEntityStore_DeleteByPredicate_NotFound(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("value", "Non-Existent")

	err = estore.DeleteByPredicate(txCtx, predicate)
	require.Error(t, err)
	assert.True(t, errors.Is(err, types.ErrNotFound), "Error should be ErrNotFound")
}

// TestPostgresEntityStore_FindByPredicate_ComplexPredicate tests FindByPredicate with complex AND predicates
func TestPostgresEntityStore_FindByPredicate_ComplexPredicate(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "complex-1",
		Value:     "Target",
		Version:   1,
		CreatedAt: time.Now(),
	}

	entity2 := &testEntity{
		ID:        "complex-2",
		Value:     "Target",
		Version:   2,
		CreatedAt: time.Now(),
	}

	entity3 := &testEntity{
		ID:        "complex-3",
		Value:     "Other",
		Version:   1,
		CreatedAt: time.Now(),
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	// Complex predicate: value = "Target" AND version = 1
	predicate := query.And(
		query.Eq("value", "Target"),
		query.Eq("version", int32(1)),
	)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Equal(t, "Target", entity.Value)
		assert.Equal(t, int64(1), entity.Version)
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresEntityStore_FindByPredicate_GreaterThan tests FindByPredicate with GreaterThan predicate
func TestPostgresEntityStore_FindByPredicate_GreaterThan(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities with different versions
	for i := int32(1); i <= 3; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "ver-" + string(rune(i)),
			Value:     "Version Test",
			Version:   int64(i),
			CreatedAt: time.Now(),
		})
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
	builder := NewPostgresJSONBBuilder()
	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Gt("version", int64(1))

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Greater(t, entity.Version, int64(1))
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_NestedJSONBPath tests FindByPredicate with nested JSONB paths
func TestPostgresEntityStore_FindByPredicate_NestedJSONBPath(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities with nested metadata
	entity1 := &testEntity{
		ID:        "nested-1",
		Value:     "Entity with nested config",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata: map[string]any{
			"config": map[string]any{
				"database": map[string]any{
					"host": "localhost",
				},
			},
		},
	}

	entity2 := &testEntity{
		ID:        "nested-2",
		Value:     "Entity with different host",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata: map[string]any{
			"config": map[string]any{
				"database": map[string]any{
					"host": "remote.example.com",
				},
			},
		},
	}

	for _, entity := range []*testEntity{entity1, entity2} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	// Query nested path
	predicate := query.Eq("metadata.config.database.host", "localhost")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.NotNil(t, entity.Metadata["config"])
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresEntityStore_FindByPredicate_IsNull_JSONB tests FindByPredicate with IsNull on JSONB field
func TestPostgresEntityStore_FindByPredicate_IsNull_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert entities with and without metadata properties
	entity1 := &testEntity{
		ID:        "null-1",
		Value:     "Entity with metadata",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"status": "active"},
	}

	entity2 := &testEntity{
		ID:        "null-2",
		Value:     "Entity without status",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"other": "value"},
	}

	for _, entity := range []*testEntity{entity1, entity2} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.IsNull("metadata.status")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresEntityStore_FindByPredicate_IsNotNull_JSONB tests FindByPredicate with IsNotNull on JSONB field
func TestPostgresEntityStore_FindByPredicate_IsNotNull_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert entities with varying metadata
	entity1 := &testEntity{
		ID:        "notnull-1",
		Value:     "Has tags",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"tags": []string{"important", "urgent"}},
	}

	entity2 := &testEntity{
		ID:        "notnull-2",
		Value:     "No tags",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"owner": "admin"},
	}

	for _, entity := range []*testEntity{entity1, entity2} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.IsNotNull("metadata.tags")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresEntityStore_FindByPredicate_NotEqual_JSONB tests FindByPredicate with NotEqual on JSONB field
func TestPostgresEntityStore_FindByPredicate_NotEqual_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "ne-1",
		Value:     "Staging env",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "staging"},
	}

	entity2 := &testEntity{
		ID:        "ne-2",
		Value:     "Prod env",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "prod"},
	}

	entity3 := &testEntity{
		ID:        "ne-3",
		Value:     "Dev env",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "dev"},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Neq("metadata.environment", "prod")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.NotEqual(t, "prod", entity.Metadata["environment"])
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_In_JSONB tests FindByPredicate with In operator on JSONB field
func TestPostgresEntityStore_FindByPredicate_In_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities with different statuses
	entity1 := &testEntity{
		ID:        "in-1",
		Value:     "Active",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"status": "active"},
	}

	entity2 := &testEntity{
		ID:        "in-2",
		Value:     "Pending",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"status": "pending"},
	}

	entity3 := &testEntity{
		ID:        "in-3",
		Value:     "Inactive",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"status": "inactive"},
	}

	entity4 := &testEntity{
		ID:        "in-4",
		Value:     "Archived",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"status": "archived"},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3, entity4} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.In("metadata.status", "active", "pending")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		status := entity.Metadata["status"].(string)
		assert.True(t, status == "active" || status == "pending")
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_NotIn_JSONB tests FindByPredicate with NotIn operator on JSONB field
func TestPostgresEntityStore_FindByPredicate_NotIn_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "notin-1",
		Value:     "Type A",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"type": "internal"},
	}

	entity2 := &testEntity{
		ID:        "notin-2",
		Value:     "Type B",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"type": "external"},
	}

	entity3 := &testEntity{
		ID:        "notin-3",
		Value:     "Type C",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"type": "deprecated"},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.NotIn("metadata.type", "deprecated", "obsolete")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		typeVal := entity.Metadata["type"].(string)
		assert.NotEqual(t, "deprecated", typeVal)
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_GreaterThan_JSONB tests FindByPredicate with GreaterThan on numeric JSONB field
func TestPostgresEntityStore_FindByPredicate_GreaterThan_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities with numeric metadata
	entity1 := &testEntity{
		ID:        "gt-1",
		Value:     "Score 50",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"score": float64(50)},
	}

	entity2 := &testEntity{
		ID:        "gt-2",
		Value:     "Score 75",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"score": float64(75)},
	}

	entity3 := &testEntity{
		ID:        "gt-3",
		Value:     "Score 90",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"score": float64(90)},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Gt("metadata.score", 60)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		score := entity.Metadata["score"].(float64)
		assert.Greater(t, score, float64(60))
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_LessThan_JSONB tests FindByPredicate with LessThan on numeric JSONB field
func TestPostgresEntityStore_FindByPredicate_LessThan_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities with numeric metadata
	entity1 := &testEntity{
		ID:        "lt-1",
		Value:     "Priority 1",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"priority": float64(1)},
	}

	entity2 := &testEntity{
		ID:        "lt-2",
		Value:     "Priority 5",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"priority": float64(5)},
	}

	entity3 := &testEntity{
		ID:        "lt-3",
		Value:     "Priority 10",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"priority": float64(10)},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Lt("metadata.priority", 6)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		priority := entity.Metadata["priority"].(float64)
		assert.Less(t, priority, float64(6))
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_GreaterEqual_JSONB tests FindByPredicate with GreaterEqual on JSONB field
func TestPostgresEntityStore_FindByPredicate_GreaterEqual_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "gte-1",
		Value:     "Value 100",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"value": float64(100)},
	}

	entity2 := &testEntity{
		ID:        "gte-2",
		Value:     "Value 150",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"value": float64(150)},
	}

	entity3 := &testEntity{
		ID:        "gte-3",
		Value:     "Value 50",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"value": float64(50)},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Gte("metadata.value", 100)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		value := entity.Metadata["value"].(float64)
		assert.GreaterOrEqual(t, value, float64(100))
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_LessEqual_JSONB tests FindByPredicate with LessEqual on JSONB field
func TestPostgresEntityStore_FindByPredicate_LessEqual_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "lte-1",
		Value:     "Count 10",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"count": float64(10)},
	}

	entity2 := &testEntity{
		ID:        "lte-2",
		Value:     "Count 25",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"count": float64(25)},
	}

	entity3 := &testEntity{
		ID:        "lte-3",
		Value:     "Count 20",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"count": float64(20)},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Lte("metadata.count", 20)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		countVal := entity.Metadata["count"].(float64)
		assert.LessOrEqual(t, countVal, float64(20))
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_FindByPredicate_CompoundAND_JSONB tests FindByPredicate with compound AND using JSONB predicates
func TestPostgresEntityStore_FindByPredicate_CompoundAND_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities with multiple metadata properties
	entity1 := &testEntity{
		ID:        "and-1",
		Value:     "Target entity",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "prod", "region": "us-east"},
	}

	entity2 := &testEntity{
		ID:        "and-2",
		Value:     "Other entity",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "prod", "region": "eu-west"},
	}

	entity3 := &testEntity{
		ID:        "and-3",
		Value:     "Another entity",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "staging", "region": "us-east"},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	// Complex predicate: environment = prod AND region = us-east
	predicate := query.And(
		query.Eq("metadata.environment", "prod"),
		query.Eq("metadata.region", "us-east"),
	)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Equal(t, "prod", entity.Metadata["environment"])
		assert.Equal(t, "us-east", entity.Metadata["region"])
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresEntityStore_FindByPredicate_CompoundOR_JSONB tests FindByPredicate with compound OR using JSONB predicates
func TestPostgresEntityStore_FindByPredicate_CompoundOR_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "or-1",
		Value:     "Prod entity",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "prod"},
	}

	entity2 := &testEntity{
		ID:        "or-2",
		Value:     "Staging entity",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "staging"},
	}

	entity3 := &testEntity{
		ID:        "or-3",
		Value:     "Dev entity",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"environment": "dev"},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	// Complex predicate: environment = prod OR environment = staging
	predicate := query.Or(
		query.Eq("metadata.environment", "prod"),
		query.Eq("metadata.environment", "staging"),
	)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		env := entity.Metadata["environment"].(string)
		assert.True(t, env == "prod" || env == "staging")
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_CountByPredicate_JSONB tests CountByPredicate with JSONB predicate
func TestPostgresEntityStore_CountByPredicate_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	for i := 1; i <= 3; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "count-jsonb-" + string(rune(i)),
			Value:     "Count JSONB Test",
			Version:   1,
			CreatedAt: time.Now(),
			Metadata:  map[string]any{"category": "database"},
		})
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("metadata.category", "database")

	count, err := estore.CountByPredicate(txCtx, predicate)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

// TestPostgresEntityStore_DeleteByPredicate_JSONB tests DeleteByPredicate with JSONB predicate
func TestPostgresEntityStore_DeleteByPredicate_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	toDelete := []*testEntity{
		{
			ID:        "del-jsonb-1",
			Value:     "Delete Me",
			Version:   1,
			CreatedAt: time.Now(),
			Metadata:  map[string]any{"temporary": true},
		},
		{
			ID:        "del-jsonb-2",
			Value:     "Delete Me",
			Version:   1,
			CreatedAt: time.Now(),
			Metadata:  map[string]any{"temporary": true},
		},
	}
	toKeep := []*testEntity{
		{
			ID:        "keep-jsonb-1",
			Value:     "Keep Me",
			Version:   1,
			CreatedAt: time.Now(),
			Metadata:  map[string]any{"temporary": false},
		},
	}

	for _, entity := range append(toDelete, toKeep...) {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("metadata.temporary", true)

	err = estore.DeleteByPredicate(txCtx, predicate)
	require.NoError(t, err)

	// Verify deletions
	var count int64
	row := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_entities WHERE id LIKE 'del-jsonb-%'")
	err = row.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Verify others still exist
	row = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_entities WHERE id LIKE 'keep-jsonb-%'")
	err = row.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

// TestPostgresEntityStore_FindFirstByPredicate_JSONB tests FindFirstByPredicate with JSONB predicate
func TestPostgresEntityStore_FindFirstByPredicate_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert multiple test entities with same metadata property
	for i := 1; i <= 3; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "first-jsonb-" + string(rune(i)),
			Value:     "First JSONB Test",
			Version:   1,
			CreatedAt: time.Now(),
			Metadata:  map[string]any{"priority": "high"},
		})
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("metadata.priority", "high")

	entity, err := estore.FindFirstByPredicate(txCtx, predicate)
	require.NoError(t, err)
	require.NotNil(t, entity)
	assert.Equal(t, "high", entity.Metadata["priority"])
}

// TestPostgresEntityStore_FindByPredicatePaginated_JSONB tests FindByPredicatePaginated with JSONB predicate
func TestPostgresEntityStore_FindByPredicatePaginated_JSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert 5 test entities with same metadata
	for i := 1; i <= 5; i++ {
		record, err := entityToRecord(&testEntity{
			ID:        "paged-jsonb-" + string(rune(i)),
			Value:     "Paginated JSONB Entity",
			Version:   1,
			CreatedAt: time.Now(),
			Metadata:  map[string]any{"status": "active"},
		})
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.Eq("metadata.status", "active")

	// Test first page
	opts := store.PaginationOptions{Limit: 2, Offset: 0}
	count := 0
	for entity, err := range estore.FindByPredicatePaginated(txCtx, predicate, opts) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Equal(t, "active", entity.Metadata["status"])
		count++
	}
	assert.Equal(t, 2, count)

	// Test second page
	opts = store.PaginationOptions{Limit: 2, Offset: 2}
	count = 0
	for entity, err := range estore.FindByPredicatePaginated(txCtx, predicate, opts) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Equal(t, "active", entity.Metadata["status"])
		count++
	}
	assert.Equal(t, 2, count)
}

// TestPostgresEntityStore_MixedPredicate_SimpleAndJSONB tests FindByPredicate with both simple column and JSONB predicates
func TestPostgresEntityStore_MixedPredicate_SimpleAndJSONB(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test entities
	entity1 := &testEntity{
		ID:        "mixed-1",
		Value:     "Mixed Value",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"owner": "admin"},
	}

	entity2 := &testEntity{
		ID:        "mixed-2",
		Value:     "Mixed Value",
		Version:   2,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"owner": "user"},
	}

	entity3 := &testEntity{
		ID:        "mixed-3",
		Value:     "Other Value",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"owner": "admin"},
	}

	for _, entity := range []*testEntity{entity1, entity2, entity3} {
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
	}

	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	// Mix simple column and JSONB predicates: value = "Mixed Value" AND metadata.owner = "admin"
	predicate := query.And(
		query.Eq("value", "Mixed Value"),
		query.Eq("metadata.owner", "admin"),
	)

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		assert.Equal(t, "Mixed Value", entity.Value)
		assert.Equal(t, "admin", entity.Metadata["owner"])
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresEntityStore_FindByPredicate_EmptyMetadata tests FindByPredicate handles empty metadata gracefully
func TestPostgresEntityStore_FindByPredicate_EmptyMetadata(t *testing.T) {
	setupEntityTable(t)
	defer CleanupTestData(t, testDB)

	// Insert entity with empty metadata
	entity := &testEntity{
		ID:        "empty-meta",
		Value:     "Empty Metadata",
		Version:   1,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{},
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
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"metadata": JSONBFieldTypeScalar,
	})

	estore := NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, builder)
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, SQLTransactionKey, tx)

	predicate := query.IsNull("metadata.nonexistent")

	count := 0
	for entity, err := range estore.FindByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		require.NotNil(t, entity)
		count++
	}

	assert.Equal(t, 1, count)
}
