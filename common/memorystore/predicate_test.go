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
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// complexEntity is a more complex test entity with multiple fields for predicate testing
type complexEntity struct {
	ID         string
	Version    int64
	Name       string
	Age        int
	Score      float64
	Active     bool
	Region     string
	Department string
}

func (e *complexEntity) GetID() string {
	return e.ID
}

func (e *complexEntity) GetVersion() int64 {
	return e.Version
}

func (e *complexEntity) IncrementVersion() {
	e.Version++
}

// setupComplexEntityStore creates a store with test data
func setupComplexEntityStore(t *testing.T) *InMemoryEntityStore[*complexEntity] {
	store := NewInMemoryEntityStore[*complexEntity]()
	ctx := context.Background()

	entities := []complexEntity{
		{ID: "1", Name: "Alice", Age: 30, Score: 95.5, Active: true, Region: "North", Department: "Engineering"},
		{ID: "2", Name: "Bob", Age: 25, Score: 87.3, Active: true, Region: "South", Department: "Sales"},
		{ID: "3", Name: "Charlie", Age: 35, Score: 92.1, Active: false, Region: "East", Department: "Engineering"},
		{ID: "4", Name: "Diana", Age: 28, Score: 88.9, Active: true, Region: "North", Department: "Marketing"},
		{ID: "5", Name: "Eve", Age: 32, Score: 91.2, Active: false, Region: "West", Department: "Engineering"},
		{ID: "6", Name: "Frank", Age: 29, Score: 89.4, Active: true, Region: "South", Department: "Sales"},
	}

	for i := range entities {
		_, err := store.Create(ctx, &entities[i])
		require.NoError(t, err)
	}

	return store
}

// TestFindByPredicate_SimplePredicate_Equal tests equality predicate
func TestFindByPredicate_SimplePredicate_Equal(t *testing.T) {
	t.Run("equal string field", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Eq("Name", "Alice")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "Alice", results[0].Name)
	})

	t.Run("equal integer field", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Eq("Age", 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, 30, results[0].Age)
	})

	t.Run("equal boolean field", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Eq("Active", true)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 4, len(results))
		for _, result := range results {
			assert.True(t, result.Active)
		}
	})

	t.Run("equal non-existing value", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Eq("Name", "NonExistent")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})
}

// TestFindByPredicate_SimplePredicate_NotEqual tests not-equal predicate
func TestFindByPredicate_SimplePredicate_NotEqual(t *testing.T) {

	t.Run("not equal string field", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Neq("Department", "Engineering")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.NotEqual(t, "Engineering", result.Department)
		}
	})

	t.Run("not equal integer field", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Neq("Age", 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 5, len(results))
		for _, result := range results {
			assert.NotEqual(t, 30, result.Age)
		}
	})

	t.Run("not equal boolean field", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Neq("Active", true)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
		for _, result := range results {
			assert.False(t, result.Active)
		}
	})
}

// TestFindByPredicate_SimplePredicate_Comparison tests comparison operators
func TestFindByPredicate_SimplePredicate_Comparison(t *testing.T) {
	t.Run("greater than integer", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Gt("Age", 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
		for _, result := range results {
			assert.Greater(t, result.Age, 30)
		}
	})

	t.Run("greater than or equal integer", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Gte("Age", 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.GreaterOrEqual(t, result.Age, 30)
		}
	})

	t.Run("less than integer", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Lt("Age", 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.Less(t, result.Age, 30)
		}
	})

	t.Run("less than or equal integer", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Lte("Age", 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 4, len(results))
		for _, result := range results {
			assert.LessOrEqual(t, result.Age, 30)
		}
	})

	t.Run("greater than float", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Gt("Score", 90.0)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.Greater(t, result.Score, 90.0)
		}
	})

	t.Run("less than or equal float", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Lte("Score", 90.0)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		for _, result := range results {
			assert.LessOrEqual(t, result.Score, 90.0)
		}
	})
}

// TestFindByPredicate_SimplePredicate_In tests IN operator
func TestFindByPredicate_SimplePredicate_In(t *testing.T) {

	t.Run("IN with multiple string values", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.In("Department", "Engineering", "Sales")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 5, len(results))
		for _, result := range results {
			assert.Contains(t, []string{"Engineering", "Sales"}, result.Department)
		}
	})

	t.Run("IN with multiple integer values", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.In("Age", 25, 30, 35)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.Contains(t, []int{25, 30, 35}, result.Age)
		}
	})

	t.Run("IN with single value", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.In("Region", "North")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
		for _, result := range results {
			assert.Equal(t, "North", result.Region)
		}
	})

	t.Run("IN with no matches", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.In("Department", "HR", "Finance")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})
}

// TestFindByPredicate_SimplePredicate_NotIn tests NOT IN operator
func TestFindByPredicate_SimplePredicate_NotIn(t *testing.T) {

	t.Run("NOT IN with multiple string values", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.NotIn("Department", "Engineering", "Sales")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		for _, result := range results {
			assert.NotContains(t, []string{"Engineering", "Sales"}, result.Department)
		}
	})

	t.Run("NOT IN with integer values", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.NotIn("Age", 25, 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		for _, result := range results {
			assert.NotContains(t, []int{25, 30}, result.Age)
		}
	})

	t.Run("NOT IN excludes all matching values", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.NotIn("Region", "North", "South", "East", "West")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})
}

// TestFindByPredicate_SimplePredicate_StringPatterns tests string pattern operators
func TestFindByPredicate_SimplePredicate_StringPatterns(t *testing.T) {

	t.Run("LIKE operator", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Like("Name", "li")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
		for _, result := range results {
			assert.Contains(t, result.Name, "li")
		}
	})

	t.Run("CONTAINS operator", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Contains("Name", "a")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		// Alice, Diana, Frank all contain 'a'
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.Contains(t, result.Name, "a")
		}
	})

	t.Run("STARTS_WITH operator", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.StartsWith("Name", "C")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "Charlie", results[0].Name)
	})

	t.Run("ENDS_WITH operator", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.EndsWith("Name", "e")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		// Alice, Charlie, Eve all end with 'e'
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.True(t, len(result.Name) > 0 && string(result.Name[len(result.Name)-1]) == "e")
		}
	})

	t.Run("CONTAINS case sensitive", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Contains("Name", "Z")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("LIKE with no matches", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Like("Name", "xyz")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})
}

// TestFindByPredicate_SimplePredicate_IsNull tests IS NULL operator
func TestFindByPredicate_SimplePredicate_IsNull(t *testing.T) {

	t.Run("IS NOT NULL always matches non-nil", func(t *testing.T) {
		store := NewInMemoryEntityStore[*complexEntity]()
		ctx := context.Background()

		// Populate with test data
		entity1 := &complexEntity{ID: "1", Name: "Alice", Age: 30}
		entity2 := &complexEntity{ID: "2", Name: "Bob", Age: 25}
		_, err := store.Create(ctx, entity1)
		require.NoError(t, err)
		_, err = store.Create(ctx, entity2)
		require.NoError(t, err)

		predicate := query.IsNotNull("Name")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		// Both entities have non-nil Name
		assert.Equal(t, 2, len(results))
	})
}

// TestFindByPredicate_CompoundPredicate_AND tests AND compound predicate
func TestFindByPredicate_CompoundPredicate_AND(t *testing.T) {
	t.Run("AND with two simple predicates", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And(
			query.Eq("Department", "Engineering"),
			query.Eq("Active", true),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "Alice", results[0].Name)
		assert.True(t, results[0].Active)
		assert.Equal(t, "Engineering", results[0].Department)
	})

	t.Run("AND with contradicting predicates", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And(
			query.Eq("Active", true),
			query.Eq("Active", false),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("AND with comparison operators", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And(
			query.Gte("Age", 28),
			query.Lte("Age", 32),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 4, len(results))
		for _, result := range results {
			assert.GreaterOrEqual(t, result.Age, 28)
			assert.LessOrEqual(t, result.Age, 32)
		}
	})

	t.Run("AND with no matching results", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And(
			query.Eq("Department", "HR"),
			query.Eq("Region", "North"),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("AND with empty predicates", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And()
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		// Empty AND should match all
		assert.Equal(t, 6, len(results))
	})
}

// TestFindByPredicate_CompoundPredicate_OR tests OR compound predicate
func TestFindByPredicate_CompoundPredicate_OR(t *testing.T) {
	t.Run("OR with multiple predicates", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Or(
			query.Eq("Department", "Engineering"),
			query.Eq("Department", "Sales"),
			query.Eq("Department", "Marketing"),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 6, len(results))
	})

	t.Run("OR with region predicates", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Or(
			query.Eq("Region", "North"),
			query.Eq("Region", "South"),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 4, len(results))
		for _, result := range results {
			assert.Contains(t, []string{"North", "South"}, result.Region)
		}
	})

	t.Run("OR with no matches", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Or(
			query.Eq("Name", "NonExistent1"),
			query.Eq("Name", "NonExistent2"),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("OR with one matching predicate", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Or(
			query.Eq("Name", "Alice"),
			query.Eq("Name", "NonExistent"),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "Alice", results[0].Name)
	})

	t.Run("OR with empty predicates", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Or()
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		// Empty should match all
		assert.Equal(t, 6, len(results))
	})
}

// TestFindByPredicate_NestedCompoundPredicate tests nested AND/OR combinations
func TestFindByPredicate_NestedCompoundPredicate(t *testing.T) {

	t.Run("nested AND within OR", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		// (Department = Engineering AND Active = true) OR (Department = Sales AND Active = true)
		predicate := query.Or(
			query.And(
				query.Eq("Department", "Engineering"),
				query.Eq("Active", true),
			),
			query.And(
				query.Eq("Department", "Sales"),
				query.Eq("Active", true),
			),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.True(t, result.Active)
			assert.Contains(t, []string{"Engineering", "Sales"}, result.Department)
		}
	})

	t.Run("nested OR within AND", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		// (Name = Alice OR Name = Bob) AND Active = true
		predicate := query.And(
			query.Or(
				query.Eq("Name", "Alice"),
				query.Eq("Name", "Bob"),
			),
			query.Eq("Active", true),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
		for _, result := range results {
			assert.True(t, result.Active)
			assert.Contains(t, []string{"Alice", "Bob"}, result.Name)
		}
	})

	t.Run("deeply nested predicates", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		// ((Department = Engineering OR Department = Sales) AND (Region = North OR Region = South)) AND Active = true
		predicate := query.And(
			query.And(
				query.Or(
					query.Eq("Department", "Engineering"),
					query.Eq("Department", "Sales"),
				),
				query.Or(
					query.Eq("Region", "North"),
					query.Eq("Region", "South"),
				),
			),
			query.Eq("Active", true),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.True(t, len(results) > 0)
		for _, result := range results {
			assert.True(t, result.Active)
			assert.Contains(t, []string{"Engineering", "Sales"}, result.Department)
			assert.Contains(t, []string{"North", "South"}, result.Region)
		}
	})
}

// TestFindByPredicate_ComplexQueries tests complex real-world queries
func TestFindByPredicate_ComplexQueries(t *testing.T) {
	t.Run("find active engineers with score above 90", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And(
			query.Eq("Department", "Engineering"),
			query.Eq("Active", true),
			query.Gt("Score", 90.0),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
		assert.Equal(t, "Alice", results[0].Name)
	})

	t.Run("find people in North or South regions younger than 30", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And(
			query.Or(
				query.Eq("Region", "North"),
				query.Eq("Region", "South"),
			),
			query.Lt("Age", 30),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))
		for _, result := range results {
			assert.Less(t, result.Age, 30)
			assert.Contains(t, []string{"North", "South"}, result.Region)
		}
	})

	t.Run("find inactive people or people from West region", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Or(
			query.Eq("Active", false),
			query.Eq("Region", "West"),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
	})

	t.Run("find salespeople with scores in a range", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.And(
			query.Eq("Department", "Sales"),
			query.Gte("Score", 87.0),
			query.Lte("Score", 90.0),
		)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
	})
}

// TestFindByPredicate_EdgeCases tests edge cases and special scenarios
func TestFindByPredicate_EdgeCases(t *testing.T) {

	t.Run("find by predicate in empty store", func(t *testing.T) {
		store := NewInMemoryEntityStore[*complexEntity]()
		ctx := context.Background()
		predicate := query.Eq("Name", "Alice")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("find by predicate with single entity", func(t *testing.T) {
		store := NewInMemoryEntityStore[*complexEntity]()
		ctx := context.Background()
		entity := &complexEntity{ID: "1", Name: "Alice", Age: 30}
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)

		predicate := query.Eq("Name", "Alice")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("find by predicate with non-matching fields", func(t *testing.T) {
		store := NewInMemoryEntityStore[*complexEntity]()
		ctx := context.Background()
		entity := &complexEntity{ID: "1", Name: "Alice", Age: 30}
		_, err := store.Create(ctx, entity)
		require.NoError(t, err)

		predicate := query.Eq("NonExistentField", "value")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("find by predicate that matches all entities", func(t *testing.T) {
		store := NewInMemoryEntityStore[*complexEntity]()
		ctx := context.Background()
		for i := 1; i <= 5; i++ {
			entity := &complexEntity{ID: string(rune('0' + i)), Name: "Name" + string(rune('0'+i)), Age: 20 + i}
			_, err := store.Create(ctx, entity)
			require.NoError(t, err)
		}

		// Use a broad predicate that matches all
		predicate := query.Gt("Age", 0)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 5, len(results))
	})
}

// TestFindByPredicate_IterationBehavior tests the iterator behavior
func TestFindByPredicate_IterationBehavior(t *testing.T) {

	t.Run("early termination of iteration", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Eq("Department", "Engineering")

		count := 0
		for _, err := range store.FindByPredicate(ctx, predicate) {
			require.NoError(t, err)
			count++
			if count == 1 {
				break // Stop after first result
			}
		}

		assert.Equal(t, 1, count)
	})

	t.Run("full iteration through results", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Eq("Active", true)

		count := 0
		for result, err := range store.FindByPredicate(ctx, predicate) {
			require.NoError(t, err)
			assert.True(t, result.Active)
			count++
		}

		assert.Equal(t, 4, count)
	})

	t.Run("multiple iterations produce same results", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Lt("Age", 30)

		// First iteration
		results1, err1 := collection.CollectAll(store.FindByPredicate(ctx, predicate))
		require.NoError(t, err1)

		// Second iteration
		results2, err2 := collection.CollectAll(store.FindByPredicate(ctx, predicate))
		require.NoError(t, err2)

		assert.Equal(t, len(results1), len(results2))
	})
}

// TestFindByPredicate_ConcurrentAccess tests thread-safe access
func TestFindByPredicate_ConcurrentAccess(t *testing.T) {

	t.Run("concurrent finds during read", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		done := make(chan bool, 3)

		// Goroutine 1: Find with predicate
		go func() {
			predicate := query.Eq("Active", true)
			results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))
			require.NoError(t, err)
			assert.Equal(t, 4, len(results))
			done <- true
		}()

		// Goroutine 2: Find with different predicate
		go func() {
			predicate := query.Eq("Department", "Engineering")
			results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))
			require.NoError(t, err)
			assert.Equal(t, 3, len(results))
			done <- true
		}()

		// Goroutine 3: Find with compound predicate
		go func() {
			predicate := query.And(
				query.Gt("Age", 25),
				query.Lt("Age", 35),
			)
			results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))
			require.NoError(t, err)
			assert.True(t, len(results) > 0)
			done <- true
		}()

		<-done
		<-done
		<-done
	})
}

// TestFindByPredicate_FieldTypeConversion tests type handling in comparisons
func TestFindByPredicate_FieldTypeConversion(t *testing.T) {

	t.Run("string field comparison with string value", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Eq("Region", "North")
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
	})

	t.Run("integer field comparison with integer value", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Eq("Age", 30)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("float field comparison with float value", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Eq("Score", 95.5)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("boolean field comparison with boolean value", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		predicate := query.Eq("Active", false)
		results, err := collection.CollectAll(store.FindByPredicate(ctx, predicate))

		require.NoError(t, err)
		assert.Equal(t, 2, len(results))
	})
}

// TestFindByPredicate_PredicateOptimization tests predicate behavior patterns
func TestFindByPredicate_PredicateOptimization(t *testing.T) {

	t.Run("selective predicates reduce results efficiently", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()
		// Less selective
		predicate1 := query.Eq("Active", true)
		results1, err := collection.CollectAll(store.FindByPredicate(ctx, predicate1))
		require.NoError(t, err)

		// More selective
		predicate2 := query.And(
			query.Eq("Active", true),
			query.Eq("Department", "Engineering"),
		)
		results2, err := collection.CollectAll(store.FindByPredicate(ctx, predicate2))
		require.NoError(t, err)

		assert.True(t, len(results2) <= len(results1))
		assert.Equal(t, 1, len(results2))
	})
}

// TestFindFirstByPredicate_SingleElement tests finding the first result from a single element
func TestFindFirstByPredicate_SingleElement(t *testing.T) {
	store := NewInMemoryEntityStore[*complexEntity]()
	ctx := context.Background()

	entity := &complexEntity{ID: "1", Name: "Alice", Age: 30, Active: true, Department: "Engineering"}
	_, err := store.Create(ctx, entity)
	require.NoError(t, err)

	predicate := query.Eq("Name", "Alice")
	result, err := store.FindFirstByPredicate(ctx, predicate)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Alice", result.Name)
}

// TestFindFirstByPredicate_MultipleElements tests finding the first result from multiple elements
func TestFindFirstByPredicate_MultipleElements(t *testing.T) {
	store := setupComplexEntityStore(t)
	ctx := context.Background()

	predicate := query.Eq("Active", true)
	result, err := store.FindFirstByPredicate(ctx, predicate)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Active)
}

// TestFindFirstByPredicate_NoResults tests finding with no matching results
func TestFindFirstByPredicate_NoResults(t *testing.T) {
	store := setupComplexEntityStore(t)
	ctx := context.Background()

	predicate := query.Eq("Name", "NonExistent")
	result, err := store.FindFirstByPredicate(ctx, predicate)

	require.Error(t, err)
	require.ErrorAs(t, err, &types.ErrNotFound)
	assert.Nil(t, result)
}

// TestCountByPredicate_SimplePredicate_Equal tests counting with equality predicate
func TestCountByPredicate_SimplePredicate_Equal(t *testing.T) {
	t.Run("count equal", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Eq("Name", "Alice")
		count, err := store.CountByPredicate(ctx, predicate)

		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("count equal non-existing value", func(t *testing.T) {
		store := setupComplexEntityStore(t)
		ctx := context.Background()

		predicate := query.Eq("Name", "NonExistent")
		count, err := store.CountByPredicate(ctx, predicate)

		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}
