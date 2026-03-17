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
	"testing"

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/query"
	store2 "github.com/eclipse-cfm/cfm/common/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindByPredicatePaginated_BasicPredicate_WithPagination tests basic predicates with pagination
func TestFindByPredicatePaginated_BasicPredicate_WithPagination(t *testing.T) {
	store := setupComplexEntityStore(t)
	ctx := context.Background()

	t.Run("find with equal predicate and limit", func(t *testing.T) {
		// 4 entities have Active=true, get first 2
		predicate := query.Eq("Active", true)
		opts := store2.PaginationOptions{Offset: 0, Limit: 2}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
		for _, result := range results {
			assert.True(t, result.Active)
		}
	})

	t.Run("find with equal predicate and offset", func(t *testing.T) {
		// 4 entities have Active=true, skip first 2, get remaining
		predicate := query.Eq("Active", true)
		opts := store2.PaginationOptions{Offset: 2, Limit: 10}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
		for _, result := range results {
			assert.True(t, result.Active)
		}
	})

	t.Run("find with equal predicate offset and limit both specified", func(t *testing.T) {
		// 3 entities in Engineering, get 2nd entity
		predicate := query.Eq("Department", "Engineering")
		opts := store2.PaginationOptions{Offset: 1, Limit: 1}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "Engineering", results[0].Department)
	})

	t.Run("find with string equal predicate and pagination", func(t *testing.T) {
		predicate := query.Eq("Region", "South")
		opts := store2.PaginationOptions{Offset: 0, Limit: 1}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "South", results[0].Region)
	})
}

// TestFindByPredicatePaginated_EdgeCases tests edge cases and boundary conditions
func TestFindByPredicatePaginated_EdgeCases(t *testing.T) {
	store := setupComplexEntityStore(t)
	ctx := context.Background()

	t.Run("offset equals number of filtered results", func(t *testing.T) {
		predicate := query.Eq("Region", "North")
		opts := store2.PaginationOptions{Offset: 2, Limit: 10} // 2 entities in North

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("negative offset treated as zero", func(t *testing.T) {
		predicate := query.Eq("Department", "Sales")
		opts := store2.PaginationOptions{Offset: -5, Limit: 1}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "Sales", results[0].Department)
	})

	t.Run("zero limit returns all remaining", func(t *testing.T) {
		// 4 entities with Active=true
		predicate := query.Eq("Active", true)
		opts := store2.PaginationOptions{Offset: 0, Limit: 0}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 4, len(results))
	})

	t.Run("limit greater than filtered results", func(t *testing.T) {
		predicate := query.Eq("Department", "Marketing")
		opts := store2.PaginationOptions{Offset: 0, Limit: 100}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("no predicate matches", func(t *testing.T) {
		predicate := query.Eq("Department", "NonExistent")
		opts := store2.PaginationOptions{Offset: 0, Limit: 10}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("empty pagination options", func(t *testing.T) {
		predicate := query.Eq("Active", true)
		opts := store2.PaginationOptions{}

		results, err := collection.CollectAll(store.FindByPredicatePaginated(ctx, predicate, opts))

		require.NoError(t, err)
		assert.Equal(t, 4, len(results))
	})
}

// TestFindByPredicatePaginated_ContextHandling tests context cancellation and timeout
func TestFindByPredicatePaginated_ContextHandling(t *testing.T) {
	store := setupComplexEntityStore(t)

	t.Run("cancelled context returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		predicate := query.Eq("Active", true)
		opts := store2.PaginationOptions{Offset: 0, Limit: 10}

		var lastErr error
		count := 0

		for _, err := range store.FindByPredicatePaginated(ctx, predicate, opts) {
			if err != nil {
				lastErr = err
				break
			}
			count++
		}

		require.Error(t, lastErr)
		assert.Equal(t, context.Canceled, lastErr)
	})

	t.Run("context timeout returns error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		predicate := query.Eq("Department", "Engineering")
		opts := store2.PaginationOptions{Offset: 0, Limit: 100}

		var lastErr error

		for _, err := range store.FindByPredicatePaginated(ctx, predicate, opts) {
			if err != nil {
				lastErr = err
				break
			}
		}

		require.Error(t, lastErr)
	})
}

// TestFindByPredicatePaginated_ConsistencyWithFindByPredicate tests consistency between methods
func TestFindByPredicatePaginated_ConsistencyWithFindByPredicate(t *testing.T) {
	store := setupComplexEntityStore(t)
	ctx := context.Background()

	t.Run("paginated with no limit equals unpaginated", func(t *testing.T) {
		predicate := query.Eq("Active", true)

		// Get all with pagination (limit=0 means no limit)
		paginatedResults, err := collection.CollectAll(
			store.FindByPredicatePaginated(ctx, predicate, store2.PaginationOptions{Offset: 0, Limit: 0}),
		)
		require.NoError(t, err)

		// Get all without pagination
		unpaginatedResults, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))
		require.NoError(t, err)

		assert.Equal(t, len(unpaginatedResults), len(paginatedResults))
	})

	t.Run("multiple paginated requests cover all results", func(t *testing.T) {
		predicate := query.Eq("Active", true) // 4 entities
		var pageSize int64 = 2

		allResults := make([]*complexEntity, 0)

		// Page 1
		results1, err := collection.CollectAll(
			store.FindByPredicatePaginated(ctx, predicate, store2.PaginationOptions{Offset: 0, Limit: pageSize}),
		)
		require.NoError(t, err)
		allResults = append(allResults, results1...)

		// Page 2
		results2, err := collection.CollectAll(
			store.FindByPredicatePaginated(ctx, predicate, store2.PaginationOptions{Offset: pageSize, Limit: pageSize}),
		)
		require.NoError(t, err)
		allResults = append(allResults, results2...)

		// Compare with unpaginated
		unpaginatedResults, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))
		require.NoError(t, err)

		assert.Equal(t, len(unpaginatedResults), len(allResults))
		assert.Equal(t, 4, len(allResults))
	})
}

// TestFindByPredicatePaginated_LargeDataset tests with larger number of entities
func TestFindByPredicatePaginated_LargeDataset(t *testing.T) {
	store := NewInMemoryEntityStore[*complexEntity]()
	ctx := context.Background()

	// Create 100 entities
	for i := 1; i <= 100; i++ {
		entity := &complexEntity{
			ID:         string(rune(i)),
			Name:       "Entity" + string(rune(i)),
			Age:        20 + i%50,
			Score:      50.0 + float64(i),
			Active:     i%2 == 0,
			Region:     "Region" + string(rune(i%5)),
			Department: "Department" + string(rune(i%3)),
		}
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)
	}

	t.Run("paginate through 50 active entities", func(t *testing.T) {
		predicate := query.Eq("Active", true)
		var pageSize int64 = 10

		allResults := make([]*complexEntity, 0)

		// Fetch all pages
		for offset := int64(0); offset < 100; offset += pageSize {
			results, err := collection.CollectAll(
				store.FindByPredicatePaginated(ctx, predicate, store2.PaginationOptions{Offset: offset, Limit: pageSize}),
			)
			require.NoError(t, err)

			if len(results) == 0 {
				break
			}

			allResults = append(allResults, results...)
		}

		// Should get exactly 50 (100/2)
		assert.Equal(t, 50, len(allResults))
		for _, result := range allResults {
			assert.True(t, result.Active)
		}
	})

	t.Run("age range with pagination", func(t *testing.T) {
		predicate := query.And(
			query.Gte("Age", 40),
			query.Lt("Age", 50),
		)

		results, err := collection.CollectAll(
			store.FindByPredicatePaginated(ctx, predicate, store2.PaginationOptions{Offset: 0, Limit: 20}),
		)
		require.NoError(t, err)

		assert.Equal(t, 20, len(results))
		for _, result := range results {
			assert.GreaterOrEqual(t, result.Age, 40)
			assert.Less(t, result.Age, 50)
		}
	})
}
