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
	"testing"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSimplePredicateSQL(t *testing.T) {
	tests := []struct {
		name         string
		predicate    *query.AtomicPredicate
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "IS NULL operator",
			predicate: &query.AtomicPredicate{
				Field:    "deleted_at",
				Operator: query.OpIsNull,
				Value:    nil,
			},
			expectedSQL:  "deleted_at IS NULL",
			expectedArgs: nil,
		},
		{
			name: "IS NOT NULL operator",
			predicate: &query.AtomicPredicate{
				Field:    "created_at",
				Operator: query.OpIsNotNull,
				Value:    nil,
			},
			expectedSQL:  "created_at IS NOT NULL",
			expectedArgs: nil,
		},
		{
			name: "EQUAL operator",
			predicate: &query.AtomicPredicate{
				Field:    "name",
				Operator: query.OpEqual,
				Value:    "John",
			},
			expectedSQL:  "name = ?",
			expectedArgs: []any{"John"},
		},
		{
			name: "NOT EQUAL operator",
			predicate: &query.AtomicPredicate{
				Field:    "status",
				Operator: query.OpNotEqual,
				Value:    "inactive",
			},
			expectedSQL:  "status != ?",
			expectedArgs: []any{"inactive"},
		},
		{
			name: "GREATER operator",
			predicate: &query.AtomicPredicate{
				Field:    "age",
				Operator: query.OpGreater,
				Value:    18,
			},
			expectedSQL:  "age > ?",
			expectedArgs: []any{18},
		},
		{
			name: "GREATER EQUAL operator",
			predicate: &query.AtomicPredicate{
				Field:    "score",
				Operator: query.OpGreaterEqual,
				Value:    100,
			},
			expectedSQL:  "score >= ?",
			expectedArgs: []any{100},
		},
		{
			name: "LESS operator",
			predicate: &query.AtomicPredicate{
				Field:    "count",
				Operator: query.OpLess,
				Value:    10,
			},
			expectedSQL:  "count < ?",
			expectedArgs: []any{10},
		},
		{
			name: "LESS EQUAL operator",
			predicate: &query.AtomicPredicate{
				Field:    "value",
				Operator: query.OpLessEqual,
				Value:    50,
			},
			expectedSQL:  "value <= ?",
			expectedArgs: []any{50},
		},
		{
			name: "LIKE operator",
			predicate: &query.AtomicPredicate{
				Field:    "email",
				Operator: query.OpLike,
				Value:    "%@example.com",
			},
			expectedSQL:  "email LIKE ?",
			expectedArgs: []any{"%@example.com"},
		},
		{
			name: "NOT LIKE operator",
			predicate: &query.AtomicPredicate{
				Field:    "domain",
				Operator: query.OpNotLike,
				Value:    "%.local",
			},
			expectedSQL:  "domain NOT LIKE ?",
			expectedArgs: []any{"%.local"},
		},
		{
			name: "IN operator with multiple values",
			predicate: &query.AtomicPredicate{
				Field:    "id",
				Operator: query.OpIn,
				Value:    []any{1, 2, 3},
			},
			expectedSQL:  "id IN (?,?,?)",
			expectedArgs: []any{1, 2, 3},
		},
		{
			name: "IN operator with single value",
			predicate: &query.AtomicPredicate{
				Field:    "type",
				Operator: query.OpIn,
				Value:    []any{"admin"},
			},
			expectedSQL:  "type IN (?)",
			expectedArgs: []any{"admin"},
		},
		{
			name: "NOT IN operator",
			predicate: &query.AtomicPredicate{
				Field:    "status",
				Operator: query.OpNotIn,
				Value:    []any{"pending", "failed", "cancelled"},
			},
			expectedSQL:  "status NOT IN (?,?,?)",
			expectedArgs: []any{"pending", "failed", "cancelled"},
		},
		{
			name: "CONTAINS operator",
			predicate: &query.AtomicPredicate{
				Field:    "description",
				Operator: query.OpContains,
				Value:    "important",
			},
			expectedSQL:  "description CONTAINS ?",
			expectedArgs: []any{"important"},
		},
		{
			name: "STARTS_WITH operator",
			predicate: &query.AtomicPredicate{
				Field:    "code",
				Operator: query.OpStartsWith,
				Value:    "ERR_",
			},
			expectedSQL:  "code STARTS_WITH ?",
			expectedArgs: []any{"ERR_"},
		},
		{
			name: "ENDS_WITH operator",
			predicate: &query.AtomicPredicate{
				Field:    "filename",
				Operator: query.OpEndsWith,
				Value:    ".txt",
			},
			expectedSQL:  "filename ENDS_WITH ?",
			expectedArgs: []any{".txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildSimplePredicateSQL(tt.predicate)

			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestBuildCompoundPredicateSQL(t *testing.T) {
	tests := []struct {
		name         string
		predicate    *query.CompoundPredicate
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name:         "Empty predicates",
			predicate:    &query.CompoundPredicate{Predicates: []query.Predicate{}, Operator: "AND"},
			expectedSQL:  "true",
			expectedArgs: nil,
		},
		{
			name: "Single predicate AND",
			predicate: &query.CompoundPredicate{
				Predicates: []query.Predicate{
					&query.AtomicPredicate{
						Field:    "name",
						Operator: query.OpEqual,
						Value:    "Alice",
					},
				},
				Operator: "AND",
			},
			expectedSQL:  "name = ?",
			expectedArgs: []any{"Alice"},
		},
		{
			name: "Two predicates AND",
			predicate: &query.CompoundPredicate{
				Predicates: []query.Predicate{
					&query.AtomicPredicate{
						Field:    "age",
						Operator: query.OpGreater,
						Value:    18,
					},
					&query.AtomicPredicate{
						Field:    "status",
						Operator: query.OpEqual,
						Value:    "active",
					},
				},
				Operator: "AND",
			},
			expectedSQL:  "(age > ?) AND (status = ?)",
			expectedArgs: []any{18, "active"},
		},
		{
			name: "Three predicates AND",
			predicate: &query.CompoundPredicate{
				Predicates: []query.Predicate{
					&query.AtomicPredicate{
						Field:    "id",
						Operator: query.OpGreater,
						Value:    0,
					},
					&query.AtomicPredicate{
						Field:    "deleted_at",
						Operator: query.OpIsNull,
						Value:    nil,
					},
					&query.AtomicPredicate{
						Field:    "role",
						Operator: query.OpEqual,
						Value:    "admin",
					},
				},
				Operator: "AND",
			},
			expectedSQL:  "(id > ?) AND (deleted_at IS NULL) AND (role = ?)",
			expectedArgs: []any{0, "admin"},
		},
		{
			name: "Two predicates OR",
			predicate: &query.CompoundPredicate{
				Predicates: []query.Predicate{
					&query.AtomicPredicate{
						Field:    "email",
						Operator: query.OpEqual,
						Value:    "alice@example.com",
					},
					&query.AtomicPredicate{
						Field:    "username",
						Operator: query.OpEqual,
						Value:    "alice",
					},
				},
				Operator: "OR",
			},
			expectedSQL:  "(email = ?) OR (username = ?)",
			expectedArgs: []any{"alice@example.com", "alice"},
		},
		{
			name: "Predicate with IN operator",
			predicate: &query.CompoundPredicate{
				Predicates: []query.Predicate{
					&query.AtomicPredicate{
						Field:    "status",
						Operator: query.OpIn,
						Value:    []any{"active", "pending"},
					},
					&query.AtomicPredicate{
						Field:    "priority",
						Operator: query.OpGreater,
						Value:    5,
					},
				},
				Operator: "AND",
			},
			expectedSQL:  "(status IN (?,?)) AND (priority > ?)",
			expectedArgs: []any{"active", "pending", 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildCompoundPredicateSQL(tt.predicate)

			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestBuildSQL(t *testing.T) {
	tests := []struct {
		name         string
		predicate    query.Predicate
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name:         "Atomic predicate - simple equality",
			predicate:    &query.AtomicPredicate{Field: "id", Operator: query.OpEqual, Value: 42},
			expectedSQL:  "id = ?",
			expectedArgs: []any{42},
		},
		{
			name: "Compound predicate - AND",
			predicate: &query.CompoundPredicate{
				Predicates: []query.Predicate{
					&query.AtomicPredicate{Field: "age", Operator: query.OpGreater, Value: 21},
					&query.AtomicPredicate{Field: "active", Operator: query.OpEqual, Value: true},
				},
				Operator: "AND",
			},
			expectedSQL:  "(age > ?) AND (active = ?)",
			expectedArgs: []any{21, true},
		},
		{
			name: "Nested compound predicates",
			predicate: &query.CompoundPredicate{
				Predicates: []query.Predicate{
					&query.CompoundPredicate{
						Predicates: []query.Predicate{
							&query.AtomicPredicate{Field: "age", Operator: query.OpGreater, Value: 18},
							&query.AtomicPredicate{Field: "age", Operator: query.OpLess, Value: 65},
						},
						Operator: "AND",
					},
					&query.AtomicPredicate{Field: "status", Operator: query.OpEqual, Value: "active"},
				},
				Operator: "AND",
			},
			expectedSQL:  "((age > ?) AND (age < ?)) AND (status = ?)",
			expectedArgs: []any{18, 65, "active"},
		},
		{
			name:         "Unknown predicate type defaults to true",
			predicate:    nil,
			expectedSQL:  "true",
			expectedArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := buildSQL(tt.predicate)

			assert.Equal(t, tt.expectedSQL, sql)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

func TestDefaultSQLBuilder(t *testing.T) {
	builder := &DefaultSQLBuilder{}

	t.Run("BuildSQL with atomic predicate", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "email",
			Operator: query.OpEqual,
			Value:    "test@example.com",
		}

		sql, args := builder.BuildSQL(predicate)

		assert.Equal(t, "email = ?", sql)
		assert.Equal(t, []any{"test@example.com"}, args)
	})

	t.Run("BuildSQL with compound predicate", func(t *testing.T) {
		predicate := &query.CompoundPredicate{
			Predicates: []query.Predicate{
				&query.AtomicPredicate{Field: "role", Operator: query.OpEqual, Value: "admin"},
				&query.AtomicPredicate{Field: "active", Operator: query.OpEqual, Value: true},
			},
			Operator: "AND",
		}

		sql, args := builder.BuildSQL(predicate)

		assert.Equal(t, "(role = ?) AND (active = ?)", sql)
		assert.Equal(t, []any{"admin", true}, args)
	})

	t.Run("BuildSQL with null predicate", func(t *testing.T) {
		sql, args := builder.BuildSQL(nil)

		assert.Equal(t, "true", sql)
		assert.Nil(t, args)
	})
}

func TestBuildSQLWithComplexScenarios(t *testing.T) {
	t.Run("Complex AND/OR combination", func(t *testing.T) {
		// (status IN ('active', 'pending') AND age > 18) OR (role = 'admin' AND deleted_at IS NULL)
		predicate := &query.CompoundPredicate{
			Predicates: []query.Predicate{
				&query.CompoundPredicate{
					Predicates: []query.Predicate{
						&query.AtomicPredicate{
							Field:    "status",
							Operator: query.OpIn,
							Value:    []any{"active", "pending"},
						},
						&query.AtomicPredicate{
							Field:    "age",
							Operator: query.OpGreater,
							Value:    18,
						},
					},
					Operator: "AND",
				},
				&query.CompoundPredicate{
					Predicates: []query.Predicate{
						&query.AtomicPredicate{
							Field:    "role",
							Operator: query.OpEqual,
							Value:    "admin",
						},
						&query.AtomicPredicate{
							Field:    "deleted_at",
							Operator: query.OpIsNull,
							Value:    nil,
						},
					},
					Operator: "AND",
				},
			},
			Operator: "OR",
		}

		sql, args := buildSQL(predicate)

		expectedSQL := "((status IN (?,?)) AND (age > ?)) OR ((role = ?) AND (deleted_at IS NULL))"
		expectedArgs := []any{"active", "pending", 18, "admin"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("NOT IN with multiple conditions", func(t *testing.T) {
		predicate := &query.CompoundPredicate{
			Predicates: []query.Predicate{
				&query.AtomicPredicate{
					Field:    "status",
					Operator: query.OpNotIn,
					Value:    []any{"deleted", "archived", "suspended"},
				},
				&query.AtomicPredicate{
					Field:    "created_at",
					Operator: query.OpGreater,
					Value:    "2024-01-01",
				},
			},
			Operator: "AND",
		}

		sql, args := buildSQL(predicate)

		expectedSQL := "(status NOT IN (?,?,?)) AND (created_at > ?)"
		expectedArgs := []any{"deleted", "archived", "suspended", "2024-01-01"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})

	t.Run("Multiple IN operators", func(t *testing.T) {
		predicate := &query.CompoundPredicate{
			Predicates: []query.Predicate{
				&query.AtomicPredicate{
					Field:    "status",
					Operator: query.OpIn,
					Value:    []any{"active", "pending"},
				},
				&query.AtomicPredicate{
					Field:    "priority",
					Operator: query.OpIn,
					Value:    []any{1, 2, 3},
				},
			},
			Operator: "AND",
		}

		sql, args := buildSQL(predicate)

		expectedSQL := "(status IN (?,?)) AND (priority IN (?,?,?))"
		expectedArgs := []any{"active", "pending", 1, 2, 3}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})
}

func TestBuildSQLArgumentOrder(t *testing.T) {
	t.Run("Arguments are appended in correct order", func(t *testing.T) {
		// Verify that arguments from multiple predicates are preserved in order
		predicate := &query.CompoundPredicate{
			Predicates: []query.Predicate{
				&query.AtomicPredicate{Field: "a", Operator: query.OpEqual, Value: "value_a"},
				&query.AtomicPredicate{Field: "b", Operator: query.OpEqual, Value: "value_b"},
				&query.AtomicPredicate{Field: "c", Operator: query.OpEqual, Value: "value_c"},
			},
			Operator: "AND",
		}

		sql, args := buildSQL(predicate)

		require.Equal(t, 3, len(args))
		assert.Equal(t, "value_a", args[0])
		assert.Equal(t, "value_b", args[1])
		assert.Equal(t, "value_c", args[2])
		assert.NotEmpty(t, sql)
	})

	t.Run("IN arguments are preserved in order", func(t *testing.T) {
		values := []any{10, 20, 30, 40, 50}
		predicate := &query.AtomicPredicate{
			Field:    "id",
			Operator: query.OpIn,
			Value:    values,
		}

		sql, args := buildSQL(predicate)

		require.Equal(t, 5, len(args))
		for i, v := range values {
			assert.Equal(t, v, args[i])
		}
		assert.NotEmpty(t, sql)
	})
}

func TestBuildSQLEdgeCases(t *testing.T) {
	t.Run("Empty IN clause", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "status",
			Operator: query.OpIn,
			Value:    []any{},
		}

		sql, args := buildSQL(predicate)

		assert.Equal(t, "status IN ()", sql)
		assert.Equal(t, []any{}, args)
	})

	t.Run("Special characters in field names", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "user_table.email_address",
			Operator: query.OpEqual,
			Value:    "test@example.com",
		}

		sql, args := buildSQL(predicate)

		assert.Equal(t, "user_table.email_address = ?", sql)
		assert.Equal(t, []any{"test@example.com"}, args)
	})

	t.Run("Null value in predicate", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "optional_field",
			Operator: query.OpEqual,
			Value:    nil,
		}

		sql, args := buildSQL(predicate)

		assert.Equal(t, "optional_field = ?", sql)
		assert.Equal(t, []any{nil}, args)
	})

	t.Run("Boolean values", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "is_verified",
			Operator: query.OpEqual,
			Value:    true,
		}

		sql, args := buildSQL(predicate)

		assert.Equal(t, "is_verified = ?", sql)
		assert.Equal(t, []any{true}, args)
	})
}

func TestBuildSQLMultipleInPredicates(t *testing.T) {
	t.Run("OR with IN predicates", func(t *testing.T) {
		predicate := &query.CompoundPredicate{
			Predicates: []query.Predicate{
				&query.AtomicPredicate{
					Field:    "status",
					Operator: query.OpIn,
					Value:    []any{"active", "pending"},
				},
				&query.AtomicPredicate{
					Field:    "role",
					Operator: query.OpIn,
					Value:    []any{"admin", "moderator"},
				},
			},
			Operator: "OR",
		}

		sql, args := buildSQL(predicate)

		expectedSQL := "(status IN (?,?)) OR (role IN (?,?))"
		expectedArgs := []any{"active", "pending", "admin", "moderator"}

		assert.Equal(t, expectedSQL, sql)
		assert.Equal(t, expectedArgs, args)
	})
}
