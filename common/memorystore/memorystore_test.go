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

package memorystore

import (
	"context"
	"fmt"
	"testing"

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/query"
	store2 "github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEntity is a simple test entity for testing the InMemoryEntityStore
type testEntity struct {
	ID      string
	Value   string
	Version int64
}

func (t *testEntity) GetID() string {
	return t.ID
}

func (t *testEntity) GetVersion() int64 {
	return t.Version
}

func (t *testEntity) IncrementVersion() {
	t.Version++
}

func TestNewInMemoryEntityStore(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()

	require.NotNil(t, store)
	require.NotNil(t, store.cache)
	assert.Equal(t, 0, len(store.cache))
}

func TestInMemoryEntityStore_Create(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	t.Run("successful create", func(t *testing.T) {
		entity := &testEntity{ID: "test-1", Value: "value1"}

		result, err := store.Create(ctx, entity)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-1", result.ID)
		assert.Equal(t, "value1", result.Value)
		assert.Equal(t, 1, len(store.cache))
	})

	t.Run("create with empty ID should fail", func(t *testing.T) {
		entity := &testEntity{ID: "", Value: "value1"}

		result, err := store.Create(ctx, entity)

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, types.ErrInvalidInput, err)
	})

	t.Run("create duplicate should fail", func(t *testing.T) {
		entity := &testEntity{ID: "test-2", Value: "value1"}
		duplicate := &testEntity{ID: "test-2", Value: "value2"}

		// First create should succeed
		result1, err1 := store.Create(ctx, entity)
		require.NoError(t, err1)
		require.NotNil(t, result1)

		// Second create with same ID should fail
		result2, err2 := store.Create(ctx, duplicate)
		require.Error(t, err2)
		require.Nil(t, result2)
		assert.Equal(t, types.ErrConflict, err2)
	})
}

func TestInMemoryEntityStore_FindById(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	t.Run("find existing entity", func(t *testing.T) {
		entity := &testEntity{ID: "test-1", Value: "value1"}
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)

		result, err := store.FindByID(ctx, "test-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-1", result.ID)
		assert.Equal(t, "value1", result.Value)
	})

	t.Run("find non-existing entity", func(t *testing.T) {
		result, err := store.FindByID(ctx, "non-existing")

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestInMemoryEntityStore_Exists(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	t.Run("entity exists", func(t *testing.T) {
		entity := &testEntity{ID: "test-1", Value: "value1"}
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)

		exists, err := store.Exists(ctx, "test-1")

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("entity does not exist", func(t *testing.T) {
		exists, err := store.Exists(ctx, "non-existing")

		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestInMemoryEntityStore_Update(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		entity := &testEntity{ID: "test-1", Value: "value1"}
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)

		updatedEntity := &testEntity{ID: "test-1", Value: "updated-value"}
		err = store.Update(ctx, updatedEntity)

		require.NoError(t, err)

		// Verify the update
		result, err := store.FindByID(ctx, "test-1")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "updated-value", result.Value)
	})

	t.Run("update entity with empty ID should fail", func(t *testing.T) {
		entity := &testEntity{ID: "", Value: "value1"}

		err := store.Update(ctx, entity)

		require.Error(t, err)
		assert.Equal(t, types.ErrInvalidInput, err)
	})

	t.Run("update non-existing entity should fail", func(t *testing.T) {
		entity := &testEntity{ID: "non-existing", Value: "value1"}

		err := store.Update(ctx, entity)

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})
}

func TestInMemoryEntityStore_Delete(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		entity := &testEntity{ID: "test-1", Value: "value1"}
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)

		err = store.Delete(ctx, "test-1")

		require.NoError(t, err)

		// Verify deletion
		result, err := store.FindByID(ctx, "test-1")
		require.Error(t, err)
		assert.Nil(t, result)

		exists, err := store.Exists(ctx, "test-1")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("delete with empty ID should fail", func(t *testing.T) {
		err := store.Delete(ctx, "")

		require.Error(t, err)
		assert.Equal(t, types.ErrInvalidInput, err)
	})

	t.Run("delete non-existing entity should fail", func(t *testing.T) {
		err := store.Delete(ctx, "non-existing")

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})
}

func TestInMemoryEntityStore_GetAll(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	t.Run("get all from empty store", func(t *testing.T) {
		entities, err := collection.CollectAll(store.GetAll(ctx))

		require.NoError(t, err)
		assert.Equal(t, 0, len(entities))
	})

	t.Run("get all entities", func(t *testing.T) {
		// Create test entities
		entities := []*testEntity{
			{ID: "test-1", Value: "value1"},
			{ID: "test-2", Value: "value2"},
			{ID: "test-3", Value: "value3"},
		}

		for _, entity := range entities {
			_, err := store.Create(ctx, entity)
			require.NoError(t, err)
		}

		result, err := collection.CollectAll(store.GetAll(ctx))

		require.NoError(t, err)
		assert.Equal(t, 3, len(result))

		// Verify all entities are present
		ids := make(map[string]bool)
		for _, entity := range result {
			ids[entity.ID] = true
		}
		assert.True(t, ids["test-1"])
		assert.True(t, ids["test-2"])
		assert.True(t, ids["test-3"])
	})
}

func TestInMemoryEntityStore_GetAllPaginated(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	// Create test entities
	entities := []*testEntity{
		{ID: "test-1", Value: "value1"},
		{ID: "test-2", Value: "value2"},
		{ID: "test-3", Value: "value3"},
		{ID: "test-4", Value: "value4"},
		{ID: "test-5", Value: "value5"},
	}

	for _, entity := range entities {
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)
	}

	t.Run("pagination with limit", func(t *testing.T) {
		opts := store2.PaginationOptions{Offset: 0, Limit: 3}
		result, err := collection.CollectAll(store.GetAllPaginated(ctx, opts))

		require.NoError(t, err)
		assert.Equal(t, 3, len(result))
	})

	t.Run("pagination with offset", func(t *testing.T) {
		opts := store2.PaginationOptions{Offset: 2, Limit: 2}
		result, err := collection.CollectAll(store.GetAllPaginated(ctx, opts))

		require.NoError(t, err)
		assert.Equal(t, 2, len(result))
	})

	t.Run("pagination with offset beyond range", func(t *testing.T) {
		opts := store2.PaginationOptions{Offset: 10, Limit: 2}
		result, err := collection.CollectAll(store.GetAllPaginated(ctx, opts))

		require.NoError(t, err)
		assert.Equal(t, 0, len(result))
	})

	t.Run("pagination with negative offset", func(t *testing.T) {
		opts := store2.PaginationOptions{Offset: -1, Limit: 2}
		result, err := collection.CollectAll(store.GetAllPaginated(ctx, opts))

		require.NoError(t, err)
		assert.Equal(t, 2, len(result))
	})

	t.Run("pagination with no limit", func(t *testing.T) {
		opts := store2.PaginationOptions{Offset: 0, Limit: 0}
		result, err := collection.CollectAll(store.GetAllPaginated(ctx, opts))

		require.NoError(t, err)
		assert.Equal(t, 5, len(result))
	})
}

func TestInMemoryEntityStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	t.Run("concurrent create and read", func(t *testing.T) {
		done := make(chan bool, 2)

		// Goroutine 1: Create entities
		go func() {
			for i := 0; i < 10; i++ {
				entity := &testEntity{ID: fmt.Sprintf("test-%d", i), Value: fmt.Sprintf("value%d", i)}
				store.Create(ctx, entity)
			}
			done <- true
		}()

		// Goroutine 2: Read entities
		go func() {
			for i := 0; i < 10; i++ {
				store.FindByID(ctx, fmt.Sprintf("test-%d", i))
			}
			done <- true
		}()

		// Wait for both goroutines
		<-done
		<-done

		// Verify some entities were created
		entities, err := collection.CollectAll(store.GetAll(ctx))
		require.NoError(t, err)
		assert.True(t, len(entities) > 0)
	})
}

func TestInMemoryEntityStore_ContextCancellation(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()

	// Create some test data
	for i := 0; i < 5; i++ {
		entity := &testEntity{ID: fmt.Sprintf("test-%d", i), Value: fmt.Sprintf("value%d", i)}
		_, err := store.Create(context.Background(), entity)
		require.NoError(t, err)
	}

	t.Run("cancelled context in GetAllPaginated", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		var results []*testEntity
		var lastErr error

		for entity, err := range store.GetAllPaginated(ctx, store2.DefaultPaginationOptions()) {
			if err != nil {
				lastErr = err
				break
			}
			results = append(results, entity)
		}

		// Should get a context cancellation error
		require.Error(t, lastErr)
		assert.Equal(t, context.Canceled, lastErr)
	})
}

func TestInMemoryEntityStore_CopyIsolation(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	entity := &testEntity{ID: "test-1", Value: "original-value"}
	_, err := store.Create(ctx, entity)
	require.NoError(t, err)

	// Retrieve the entity (first retrieval)
	retrieved1, err := store.FindByID(ctx, "test-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved1)
	assert.Equal(t, "original-value", retrieved1.Value)

	// Modify the retrieved copy
	retrieved1.Value = "modified-value"

	// Retrieve the entity again (second retrieval)
	retrieved2, err := store.FindByID(ctx, "test-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved2)

	// The second retrieval should still have the original value
	// The modification to retrieved1 should not be visible in retrieved2
	assert.Equal(t, "original-value", retrieved2.Value)
	assert.NotEqual(t, retrieved1.Value, retrieved2.Value)
}

func TestDeleteByPredicate(t *testing.T) {
	type args struct {
		entities  []testEntity
		predicate query.Predicate
	}

	tests := []struct {
		name             string
		args             args
		wantDeletedCount int
		wantRemaining    int
		wantErr          bool
	}{
		{
			name: "delete single entity matching predicate",
			args: args{
				entities: []testEntity{
					{ID: "1", Value: "apple"},
					{ID: "2", Value: "banana"},
					{ID: "3", Value: "cherry"},
				},
				predicate: query.Eq("Value", "banana"),
			},
			wantDeletedCount: 1,
			wantRemaining:    2,
			wantErr:          false,
		},
		{
			name: "delete multiple entities matching predicate",
			args: args{
				entities: []testEntity{
					{ID: "1", Value: "test"},
					{ID: "2", Value: "test"},
					{ID: "3", Value: "other"},
				},
				predicate: query.Eq("Value", "test"),
			},
			wantDeletedCount: 2,
			wantRemaining:    1,
			wantErr:          false,
		},
		{
			name: "delete no entities when predicate matches nothing",
			args: args{
				entities: []testEntity{
					{ID: "1", Value: "apple"},
					{ID: "2", Value: "banana"},
				},
				predicate: query.Eq("Value", "nonexistent"),
			},
			wantDeletedCount: 0,
			wantRemaining:    2,
			wantErr:          false,
		},
		{
			name: "delete all entities matching predicate",
			args: args{
				entities: []testEntity{
					{ID: "1", Value: "delete_me"},
					{ID: "2", Value: "delete_me"},
					{ID: "3", Value: "delete_me"},
				},
				predicate: query.Eq("Value", "delete_me"),
			},
			wantDeletedCount: 3,
			wantRemaining:    0,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewInMemoryEntityStore[*testEntity]()

			for _, entity := range tt.args.entities {
				e := entity
				_, err := store.Create(context.Background(), &e)
				if err != nil {
					t.Fatalf("failed to create entity: %v", err)
				}
			}

			err := store.DeleteByPredicate(context.Background(), tt.args.predicate)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteByPredicate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			count := 0
			for _, err := range store.GetAll(context.Background()) {
				if err != nil {
					t.Fatalf("error iterating: %v", err)
				}
				count++
			}

			if count != tt.wantRemaining {
				t.Errorf("DeleteByPredicate() remaining = %d, want %d", count, tt.wantRemaining)
			}
		})
	}
}

func TestInMemoryEntityStore_Update_VersionIncremented(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	// Create an entity
	entity := &testEntity{
		ID:    "test-id",
		Value: "initial data",
	}

	_, err := store.Create(ctx, entity)
	require.NoError(t, err, "Create should succeed")

	// Verify initial version
	retrieved, err := store.FindByID(ctx, "test-id")
	require.NoError(t, err, "FindByID should succeed")
	assert.Equal(t, int64(0), retrieved.GetVersion(), "Initial version should be 0")

	// Update the entity
	entity.Value = "updated data"
	err = store.Update(ctx, entity)
	require.NoError(t, err, "Update should succeed")

	// Verify version is incremented
	updated, err := store.FindByID(ctx, "test-id")
	require.NoError(t, err, "find should succeed after update")
	assert.Equal(t, int64(1), updated.GetVersion(), "Version should be incremented to 1 after update")
}

func TestInMemoryEntityStore_GetAllCount(t *testing.T) {
	store := NewInMemoryEntityStore[*testEntity]()
	ctx := context.Background()

	// Test empty store
	count, err := store.GetAllCount(ctx)
	if err != nil {
		t.Fatalf("GetAllCount failed on empty store: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count to be 0, got %d", count)
	}

	// Create and add multiple entities
	entity1 := &testEntity{ID: "entity-1", Value: "value1", Version: 1}
	entity2 := &testEntity{ID: "entity-2", Value: "value2", Version: 1}
	entity3 := &testEntity{ID: "entity-3", Value: "value3", Version: 1}

	_, err = store.Create(ctx, entity1)
	if err != nil {
		t.Fatalf("failed to create entity1: %v", err)
	}

	_, err = store.Create(ctx, entity2)
	if err != nil {
		t.Fatalf("failed to create entity2: %v", err)
	}

	// Test count after adding 2 entities
	count, err = store.GetAllCount(ctx)
	if err != nil {
		t.Fatalf("GetAllCount failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count to be 2, got %d", count)
	}

	// Add one more entity
	_, err = store.Create(ctx, entity3)
	if err != nil {
		t.Fatalf("failed to create entity3: %v", err)
	}

	// Test count after adding 3 entities
	count, err = store.GetAllCount(ctx)
	if err != nil {
		t.Fatalf("GetAllCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count to be 3, got %d", count)
	}

	// Test count after deleting an entity
	err = store.Delete(ctx, "entity-2")
	if err != nil {
		t.Fatalf("failed to delete entity2: %v", err)
	}

	count, err = store.GetAllCount(ctx)
	if err != nil {
		t.Fatalf("GetAllCount failed after delete: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count to be 2 after delete, got %d", count)
	}
}
