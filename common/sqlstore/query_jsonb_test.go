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

func TestPostgreSQLJSONBBuilder_Initialization(t *testing.T) {
	builder := NewPostgresJSONBBuilder()
	require.NotNil(t, builder)
	require.NotNil(t, builder.jsonbFields)
	assert.Empty(t, builder.jsonbFields)
}

func TestPostgreSQLJSONBBuilder_WithJSONBFields(t *testing.T) {
	builder := NewPostgresJSONBBuilder()
	result := builder.WithJSONBFields("VPAs", "Properties")
	require.NotNil(t, result)
	require.IsType(t, (*PostgresJSONBBuilder)(nil), result)

	b := result.(*PostgresJSONBBuilder)
	assert.True(t, b.jsonbFields["vpas"])
	assert.True(t, b.jsonbFields["properties"])
}

func TestPostgreSQLJSONBBuilder_WithJSONBFieldsChaining(t *testing.T) {
	result := NewPostgresJSONBBuilder().
		WithJSONBFields("VPAs").
		WithJSONBFields("Properties")

	b := result.(*PostgresJSONBBuilder)
	assert.True(t, b.jsonbFields["vpas"])
	assert.True(t, b.jsonbFields["properties"])
}

func TestPostgreSQLJSONBBuilder_WithJSONBFieldTypes(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"VPAs":   JSONBFieldTypeArrayOfObjects,
		"Tags":   JSONBFieldTypeArrayOfScalars,
		"Config": JSONBFieldTypeScalar,
	})

	b := builder.(*PostgresJSONBBuilder)
	assert.True(t, b.jsonbFields["vpas"])
	assert.Equal(t, JSONBFieldTypeArrayOfObjects, b.jsonbFieldTypes["vpas"])
	assert.Equal(t, JSONBFieldTypeArrayOfScalars, b.jsonbFieldTypes["tags"])
	assert.Equal(t, JSONBFieldTypeScalar, b.jsonbFieldTypes["config"])
}

func TestPostgreSQLJSONBBuilder_SimpleEqualityPath_ArrayOfObjects(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpEqual,
		Value:    "connector",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "connector", args[0])
}

func TestPostgreSQLJSONBBuilder_SimpleEqualityPath_Scalar(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Config": JSONBFieldTypeScalar,
	})

	predicate := &query.AtomicPredicate{
		Field:    "config.debug",
		Operator: query.OpEqual,
		Value:    true,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "config->>'debug' = $1", sql)
	require.Len(t, args, 1)
	assert.Equal(t, true, args[0])
}

func TestPostgreSQLJSONBBuilder_SimpleEqualityPath_ArrayOfScalars(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Tags": JSONBFieldTypeArrayOfScalars,
	})

	predicate := &query.AtomicPredicate{
		Field:    "Tags",
		Operator: query.OpEqual,
		Value:    "important",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(tags) elem WHERE elem = to_jsonb($1::text))", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "important", args[0])
}

func TestPostgreSQLJSONBBuilder_NestedPath(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.cell.id",
		Operator: query.OpEqual,
		Value:    "cell1",
	}

	sql, args := builder.BuildSQL(predicate)

	expected := "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->'cell'->>'id' = $1)"
	assert.Equal(t, expected, sql)
	require.Len(t, args, 1)
	assert.Equal(t, "cell1", args[0])
}

func TestPostgreSQLJSONBBuilder_IsNull(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpIsNull,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' IS NULL)", sql)
	assert.Nil(t, args)
}

func TestPostgreSQLJSONBBuilder_IsNull_Scalar(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Config": JSONBFieldTypeScalar,
	})

	predicate := &query.AtomicPredicate{
		Field:    "config.value",
		Operator: query.OpIsNull,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "config->'value' IS NULL", sql)
	assert.Nil(t, args)
}

func TestPostgreSQLJSONBBuilder_IsNotNull(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpIsNotNull,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' IS NOT NULL)", sql)
	assert.Nil(t, args)
}

func TestPostgreSQLJSONBBuilder_NotEqual(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpNotEqual,
		Value:    "old-connector",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' != $1)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "old-connector", args[0])
}

func TestPostgreSQLJSONBBuilder_NotEqual_Scalar(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Config": JSONBFieldTypeScalar,
	})

	predicate := &query.AtomicPredicate{
		Field:    "config.status",
		Operator: query.OpNotEqual,
		Value:    "disabled",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "config->>'status' != $1", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "disabled", args[0])
}

func TestPostgreSQLJSONBBuilder_InOperator(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpIn,
		Value:    []any{"connector", "credential-service"},
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' IN ($1,$2))", sql)
	require.Len(t, args, 2)
	assert.Equal(t, "connector", args[0])
	assert.Equal(t, "credential-service", args[1])
}

func TestPostgreSQLJSONBBuilder_InOperator_ArrayOfScalars(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Tags": JSONBFieldTypeArrayOfScalars,
	})

	predicate := &query.AtomicPredicate{
		Field:    "Tags",
		Operator: query.OpIn,
		Value:    []any{"critical", "urgent"},
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(tags) elem WHERE elem::text IN ($1,$2))", sql)
	require.Len(t, args, 2)
	assert.Equal(t, "critical", args[0])
	assert.Equal(t, "urgent", args[1])
}

func TestPostgreSQLJSONBBuilder_GreaterThan(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.version",
		Operator: query.OpGreater,
		Value:    1,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE (elem->'version')::numeric > $1::numeric)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, 1, args[0])
}

func TestPostgreSQLJSONBBuilder_GreaterThan_Scalar(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Metrics": JSONBFieldTypeScalar,
	})

	predicate := &query.AtomicPredicate{
		Field:    "metrics.count",
		Operator: query.OpGreater,
		Value:    100,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "(metrics->>'count')::numeric > $1::numeric", sql)
	require.Len(t, args, 1)
	assert.Equal(t, 100, args[0])
}

func TestPostgreSQLJSONBBuilder_LessThan(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.priority",
		Operator: query.OpLess,
		Value:    10,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE (elem->'priority')::numeric < $1::numeric)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, 10, args[0])
}

func TestPostgreSQLJSONBBuilder_GreaterEqual(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.count",
		Operator: query.OpGreaterEqual,
		Value:    5,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE (elem->'count')::numeric >= $1::numeric)", sql)
	require.Len(t, args, 1)
}

func TestPostgreSQLJSONBBuilder_LessEqual(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.maxCount",
		Operator: query.OpLessEqual,
		Value:    100,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE (elem->'maxCount')::numeric <= $1::numeric)", sql)
	require.Len(t, args, 1)
}

func TestPostgreSQLJSONBBuilder_NotEqual_NestedPath(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.cell.state",
		Operator: query.OpNotEqual,
		Value:    "inactive",
	}

	sql, args := builder.BuildSQL(predicate)

	expected := "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->'cell'->>'state' != $1)"
	assert.Equal(t, expected, sql)
	require.Len(t, args, 1)
	assert.Equal(t, "inactive", args[0])
}

func TestPostgreSQLJSONBBuilder_NonJSONBField(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "TenantID",
		Operator: query.OpEqual,
		Value:    "tenant1",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "TenantID = $1", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "tenant1", args[0])
}

func TestPostgreSQLJSONBBuilder_CompoundAND(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	compound := &query.CompoundPredicate{
		Operator: "AND",
		Predicates: []query.Predicate{
			&query.AtomicPredicate{
				Field:    "vpas.type",
				Operator: query.OpEqual,
				Value:    "connector",
			},
			&query.AtomicPredicate{
				Field:    "vpas.cell.id",
				Operator: query.OpEqual,
				Value:    "cell1",
			},
		},
	}

	sql, args := builder.BuildSQL(compound)

	expected := "(EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)) AND (EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->'cell'->>'id' = $2))"
	assert.Equal(t, expected, sql)
	require.Len(t, args, 2)
	assert.Equal(t, "connector", args[0])
	assert.Equal(t, "cell1", args[1])
}

func TestPostgreSQLJSONBBuilder_CompoundOR(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	compound := &query.CompoundPredicate{
		Operator: "OR",
		Predicates: []query.Predicate{
			&query.AtomicPredicate{
				Field:    "vpas.type",
				Operator: query.OpEqual,
				Value:    "connector",
			},
			&query.AtomicPredicate{
				Field:    "vpas.type",
				Operator: query.OpEqual,
				Value:    "credential-service",
			},
		},
	}

	sql, args := builder.BuildSQL(compound)

	expected := "(EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)) OR (EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $2))"
	assert.Equal(t, expected, sql)
	require.Len(t, args, 2)
	assert.Equal(t, "connector", args[0])
	assert.Equal(t, "credential-service", args[1])
}

func TestPostgreSQLJSONBBuilder_BuildJSONBPath(t *testing.T) {
	builder := NewPostgresJSONBBuilder()

	tests := []struct {
		name     string
		field    string
		path     []string
		expected string
	}{
		{
			name:     "single level",
			field:    "vpas",
			path:     []string{"type"},
			expected: "vpas->'type'",
		},
		{
			name:     "two levels",
			field:    "vpas",
			path:     []string{"cell", "id"},
			expected: "vpas->'cell'->'id'",
		},
		{
			name:     "three levels",
			field:    "properties",
			path:     []string{"config", "deployment", "cluster"},
			expected: "properties->'config'->'deployment'->'cluster'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.buildJSONBPath(tt.field, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgreSQLJSONBBuilder_BuildJSONBAccessor(t *testing.T) {
	builder := NewPostgresJSONBBuilder()

	tests := []struct {
		name     string
		field    string
		path     []string
		textCast bool
		expected string
	}{
		{
			name:     "single level preserves case",
			field:    "vpas",
			path:     []string{"Type"},
			textCast: true,
			expected: "vpas->>'Type'",
		},
		{
			name:     "single level with text cast",
			field:    "vpas",
			path:     []string{"type"},
			textCast: true,
			expected: "vpas->>'type'",
		},
		{
			name:     "single level without text cast",
			field:    "vpas",
			path:     []string{"type"},
			textCast: false,
			expected: "vpas->'type'",
		},
		{
			name:     "two levels with text cast",
			field:    "vpas",
			path:     []string{"cell", "id"},
			textCast: true,
			expected: "vpas->'cell'->>'id'",
		},
		{
			name:     "two levels without text cast",
			field:    "vpas",
			path:     []string{"cell", "id"},
			textCast: false,
			expected: "vpas->'cell'->'id'",
		},
		{
			name:     "empty path",
			field:    "vpas",
			path:     []string{},
			textCast: true,
			expected: "vpas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.buildJSONBAccessor(tt.field, tt.path, tt.textCast)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgreSQLJSONBBuilder_BuildArrayElementAccessor(t *testing.T) {
	builder := NewPostgresJSONBBuilder()

	tests := []struct {
		name     string
		path     []string
		textCast bool
		expected string
	}{
		{
			name:     "empty path with text cast",
			path:     []string{},
			textCast: true,
			expected: "elem::text",
		},
		{
			name:     "empty path without text cast",
			path:     []string{},
			textCast: false,
			expected: "elem",
		},
		{
			name:     "single level with text cast",
			path:     []string{"type"},
			textCast: true,
			expected: "elem->>'type'",
		},
		{
			name:     "two levels with text cast",
			path:     []string{"cell", "id"},
			textCast: true,
			expected: "elem->'cell'->>'id'",
		},
		{
			name:     "two levels without text cast",
			path:     []string{"cell", "id"},
			textCast: false,
			expected: "elem->'cell'->'id'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.buildArrayElementAccessor(tt.path, tt.textCast)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostgreSQLJSONBBuilder_BuildJSONBArraySearch(t *testing.T) {
	builder := NewPostgresJSONBBuilder()

	tests := []struct {
		name      string
		field     string
		path      []string
		operator  string
		paramNum  int
		fieldType JSONBFieldType
		expected  string
	}{
		{
			name:      "single level",
			field:     "vpas",
			path:      []string{"type"},
			operator:  "=",
			paramNum:  1,
			fieldType: JSONBFieldTypeArrayOfObjects,
			expected:  "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)",
		},
		{
			name:      "two levels",
			field:     "vpas",
			path:      []string{"cell", "id"},
			operator:  "=",
			paramNum:  1,
			fieldType: JSONBFieldTypeArrayOfObjects,
			expected:  "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->'cell'->>'id' = $1)",
		},
		{
			name:      "not equal operator",
			field:     "vpas",
			path:      []string{"state"},
			operator:  "!=",
			paramNum:  1,
			fieldType: JSONBFieldTypeArrayOfObjects,
			expected:  "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'state' != $1)",
		},
		{
			name:      "three levels",
			field:     "config",
			path:      []string{"deployment", "cluster", "name"},
			operator:  "=",
			paramNum:  1,
			fieldType: JSONBFieldTypeArrayOfObjects,
			expected:  "EXISTS (SELECT 1 FROM jsonb_array_elements(config) elem WHERE elem->'deployment'->'cluster'->>'name' = $1)",
		},
		{
			name:      "array of scalars no path",
			field:     "tags",
			path:      []string{},
			operator:  "=",
			paramNum:  1,
			fieldType: JSONBFieldTypeArrayOfScalars,
			expected:  "EXISTS (SELECT 1 FROM jsonb_array_elements(tags) elem WHERE elem = to_jsonb($1::text))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := builder.buildJSONBArraySearch(tt.field, tt.path, tt.operator, tt.paramNum, tt.fieldType)
			assert.Equal(t, tt.expected, sql)
		})
	}
}

func TestPostgreSQLJSONBBuilder_CaseInsensitiveFields(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs", "Properties")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpEqual,
		Value:    "connector",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "connector", args[0])
}

func TestPostgreSQLJSONBBuilder_EdgeCaseEmptyString(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpEqual,
		Value:    "",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "", args[0])
}

func TestPostgreSQLJSONBBuilder_EdgeCaseNilValue(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.type",
		Operator: query.OpEqual,
		Value:    nil,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)", sql)
	require.Len(t, args, 1)
	assert.Nil(t, args[0])
}

func TestPostgreSQLJSONBBuilder_EdgeCaseNumeric(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.version",
		Operator: query.OpEqual,
		Value:    42,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'version' = $1)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, 42, args[0])
}

func TestPostgreSQLJSONBBuilder_RealWorldScenario_VPAInCell(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs", "Properties")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.cell.id",
		Operator: query.OpEqual,
		Value:    "cell1",
	}

	sql, args := builder.BuildSQL(predicate)

	expected := "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->'cell'->>'id' = $1)"
	assert.Equal(t, expected, sql)
	require.Len(t, args, 1)
	assert.Equal(t, "cell1", args[0])
}

func TestPostgreSQLJSONBBuilder_RealWorldScenario_TypeAndCell(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	compound := &query.CompoundPredicate{
		Operator: "AND",
		Predicates: []query.Predicate{
			&query.AtomicPredicate{
				Field:    "vpas.type",
				Operator: query.OpEqual,
				Value:    "connector",
			},
			&query.AtomicPredicate{
				Field:    "vpas.cell.id",
				Operator: query.OpEqual,
				Value:    "cell1",
			},
		},
	}

	sql, args := builder.BuildSQL(compound)

	expected := "(EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)) AND (EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->'cell'->>'id' = $2))"
	assert.Equal(t, expected, sql)
	require.Len(t, args, 2)
	assert.Equal(t, "connector", args[0])
	assert.Equal(t, "cell1", args[1])
}

func TestPostgreSQLJSONBBuilder_EmptyCompound(t *testing.T) {
	builder := NewPostgresJSONBBuilder()

	compound := &query.CompoundPredicate{
		Operator:   "AND",
		Predicates: []query.Predicate{},
	}

	sql, args := builder.BuildSQL(compound)

	assert.Equal(t, "true", sql)
	assert.Nil(t, args)
}

func TestPostgreSQLJSONBBuilder_SinglePredicateCompound(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	compound := &query.CompoundPredicate{
		Operator: "AND",
		Predicates: []query.Predicate{
			&query.AtomicPredicate{
				Field:    "vpas.type",
				Operator: query.OpEqual,
				Value:    "connector",
			},
		},
	}

	sql, args := builder.BuildSQL(compound)

	// Should simplify to single predicate
	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'type' = $1)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "connector", args[0])
}

func TestPostgreSQLJSONBBuilder_Contains(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Properties": JSONBFieldTypeScalar,
	})

	predicate := &query.AtomicPredicate{
		Field:    "Properties",
		Operator: query.OpContains,
		Value:    `{"env":"prod"}`,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "properties @> $1", sql)
	require.Len(t, args, 1)
	assert.Equal(t, `{"env":"prod"}`, args[0])
}

func TestPostgreSQLJSONBBuilder_Contains_ArrayOfObjects(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "VPAs",
		Operator: query.OpContains,
		Value:    `{"Type":"connector"}`,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "vpas @> $1", sql)
	require.Len(t, args, 1)
	assert.Equal(t, `{"Type":"connector"}`, args[0])
}

func TestPostgreSQLJSONBBuilder_NotIn(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")

	predicate := &query.AtomicPredicate{
		Field:    "vpas.status",
		Operator: query.OpNotIn,
		Value:    []any{"deleted", "archived"},
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(vpas) elem WHERE elem->>'status' NOT IN ($1,$2))", sql)
	require.Len(t, args, 2)
	assert.Equal(t, "deleted", args[0])
	assert.Equal(t, "archived", args[1])
}

func TestPostgreSQLJSONBBuilder_NotIn_ArrayOfScalars(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Tags": JSONBFieldTypeArrayOfScalars,
	})

	predicate := &query.AtomicPredicate{
		Field:    "Tags",
		Operator: query.OpNotIn,
		Value:    []any{"deprecated", "obsolete"},
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(tags) elem WHERE elem::text NOT IN ($1,$2))", sql)
	require.Len(t, args, 2)
	assert.Equal(t, "deprecated", args[0])
	assert.Equal(t, "obsolete", args[1])
}

func TestPostgreSQLJSONBBuilder_DeepNestedPath(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFields("Config")

	predicate := &query.AtomicPredicate{
		Field:    "Config.Database.Connection.Host",
		Operator: query.OpEqual,
		Value:    "localhost",
	}

	sql, args := builder.BuildSQL(predicate)

	expected := "EXISTS (SELECT 1 FROM jsonb_array_elements(config) elem WHERE elem->'Database'->'Connection'->>'Host' = $1)"
	assert.Equal(t, expected, sql)
	require.Len(t, args, 1)
	assert.Equal(t, "localhost", args[0])
}

func TestPostgreSQLJSONBBuilder_ScalarFieldDirectAccess(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Metadata": JSONBFieldTypeScalar,
	})

	predicate := &query.AtomicPredicate{
		Field:    "Metadata",
		Operator: query.OpEqual,
		Value:    `{"version":1}`,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "metadata = $1", sql)
	require.Len(t, args, 1)
	assert.Equal(t, `{"version":1}`, args[0])
}

func TestPostgreSQLJSONBBuilder_ArrayOfScalars_DirectAccess(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Categories": JSONBFieldTypeArrayOfScalars,
	})

	predicate := &query.AtomicPredicate{
		Field:    "Categories",
		Operator: query.OpEqual,
		Value:    "finance",
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(categories) elem WHERE elem = to_jsonb($1::text))", sql)
	require.Len(t, args, 1)
	assert.Equal(t, "finance", args[0])
}

func TestPostgreSQLJSONBBuilder_GreaterThan_ArrayOfScalars(t *testing.T) {
	builder := NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]JSONBFieldType{
		"Scores": JSONBFieldTypeArrayOfScalars,
	})

	predicate := &query.AtomicPredicate{
		Field:    "Scores",
		Operator: query.OpGreater,
		Value:    80,
	}

	sql, args := builder.BuildSQL(predicate)

	assert.Equal(t, "EXISTS (SELECT 1 FROM jsonb_array_elements(scores) elem WHERE elem::text::numeric > $1::numeric)", sql)
	require.Len(t, args, 1)
	assert.Equal(t, 80, args[0])
}
