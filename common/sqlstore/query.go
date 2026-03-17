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
	"fmt"
	"strings"

	"github.com/eclipse-cfm/cfm/common/query"
)

const (
	// JSONBFieldTypeUnknown means we'll try to determine the best approach
	JSONBFieldTypeUnknown JSONBFieldType = iota
	// JSONBFieldTypeScalar is for scalar JSONB values (direct accessor)
	JSONBFieldTypeScalar
	// JSONBFieldTypeArrayOfObjects is for arrays of objects (needs array_elements)
	JSONBFieldTypeArrayOfObjects
	// JSONBFieldTypeArrayOfScalars is for arrays of scalar values (needs array_elements)
	JSONBFieldTypeArrayOfScalars
	// JSONBFieldTypeMapOfArrays is for maps where values are arrays (e.g. map[string][]string)
	JSONBFieldTypeMapOfArrays
)

// SQLBuilder converts predicates to SQL
// This allows different SQL dialects or custom SQL generation strategies
type SQLBuilder interface {
	// BuildSQL converts a predicate to SQL WHERE clause and arguments
	BuildSQL(predicate query.Predicate) (string, []any)
}

// buildSQL is the core SQL generation logic
func buildSQL(predicate query.Predicate) (string, []any) {
	switch p := predicate.(type) {
	case *query.AtomicPredicate:
		return buildSimplePredicateSQL(p)
	case *query.CompoundPredicate:
		return buildCompoundPredicateSQL(p)
	default:
		return "true", nil
	}
}

func buildSimplePredicateSQL(predicate *query.AtomicPredicate) (string, []any) {
	switch predicate.Operator {
	case query.OpIsNull:
		return fmt.Sprintf("%s IS NULL", predicate.Field), nil
	case query.OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", predicate.Field), nil
	case query.OpIn, query.OpNotIn:
		values := predicate.Value.([]any)
		placeholders := make([]string, len(values))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		sql := fmt.Sprintf("%s %s (%s)", predicate.Field, predicate.Operator, strings.Join(placeholders, ","))
		return sql, values
	default:
		return fmt.Sprintf("%s %s ?", predicate.Field, predicate.Operator), []any{predicate.Value}
	}
}

func buildCompoundPredicateSQL(predicate *query.CompoundPredicate) (string, []any) {
	if len(predicate.Predicates) == 0 {
		return "true", nil
	}
	if len(predicate.Predicates) == 1 {
		return buildSQL(predicate.Predicates[0])
	}

	parts := make([]string, len(predicate.Predicates))
	var args []any

	for i, pred := range predicate.Predicates {
		sql, predArgs := buildSQL(pred)
		parts[i] = fmt.Sprintf("(%s)", sql)
		args = append(args, predArgs...)
	}

	return strings.Join(parts, fmt.Sprintf(" %s ", predicate.Operator)), args
}

// DefaultSQLBuilder is the default SQL builder using ANSI SQL
type DefaultSQLBuilder struct{}

func (b *DefaultSQLBuilder) BuildSQL(predicate query.Predicate) (string, []any) {
	return buildSQL(predicate)
}

// JSONBFieldType specifies how a JSONB field should be handled
type JSONBFieldType int

// JSONBSQLBuilder builds SQL queries with JSONB field support
type JSONBSQLBuilder interface {
	// WithJSONBFields configures which fields are stored as JSONB columns
	WithJSONBFields(fields ...string) JSONBSQLBuilder

	// WithJSONBFieldTypes configures JSONB field types
	WithJSONBFieldTypes(fieldTypes map[string]JSONBFieldType) JSONBSQLBuilder

	// WithFieldMappings configures field name to column name mappings
	WithFieldMappings(mappings map[string]string) JSONBSQLBuilder

	// BuildSQL converts a predicate to SQL WHERE clause and arguments
	BuildSQL(predicate query.Predicate) (string, []any)
}

// PostgresJSONBBuilder handles SQL generation with JSONB field support
// This builder is stateless and thread-safe
type PostgresJSONBBuilder struct {
	jsonbFields     map[string]bool
	jsonbFieldTypes map[string]JSONBFieldType
	fieldMappings   map[string]string
}

// NewPostgresJSONBBuilder creates a new JSONB-aware SQL builder
func NewPostgresJSONBBuilder() *PostgresJSONBBuilder {
	return &PostgresJSONBBuilder{
		jsonbFields:     make(map[string]bool),
		jsonbFieldTypes: make(map[string]JSONBFieldType),
		fieldMappings:   make(map[string]string),
	}
}

// WithJSONBFields configures JSONB fields (defaults to array of objects)
func (b *PostgresJSONBBuilder) WithJSONBFields(fields ...string) JSONBSQLBuilder {
	for _, field := range fields {
		lowerField := strings.ToLower(field)
		b.jsonbFields[lowerField] = true
		// Default to array of objects if not explicitly set
		if _, exists := b.jsonbFieldTypes[lowerField]; !exists {
			b.jsonbFieldTypes[lowerField] = JSONBFieldTypeArrayOfObjects
		}
	}
	return b
}

// WithJSONBFieldTypes configures JSONB field types
func (b *PostgresJSONBBuilder) WithJSONBFieldTypes(fieldTypes map[string]JSONBFieldType) JSONBSQLBuilder {
	for field, fieldType := range fieldTypes {
		lowerField := strings.ToLower(field)
		b.jsonbFieldTypes[lowerField] = fieldType
		b.jsonbFields[lowerField] = true
	}
	return b
}

func (b *PostgresJSONBBuilder) WithFieldMappings(mappings map[string]string) JSONBSQLBuilder {
	for k, v := range mappings {
		b.fieldMappings[k] = v
	}
	return b
}

// BuildSQL converts a predicate to SQL WHERE clause with JSONB support
func (b *PostgresJSONBBuilder) BuildSQL(predicate query.Predicate) (string, []any) {
	paramCounter := 0
	sql, args, _ := b.buildSQL(predicate, &paramCounter)
	return sql, args
}

// buildSQL is the internal method that maintains parameter counter through recursion
func (b *PostgresJSONBBuilder) buildSQL(predicate query.Predicate, paramCounter *int) (string, []any, int) {
	switch p := predicate.(type) {
	case *query.AtomicPredicate:
		return b.buildSimplePredicateSQL(p, paramCounter)
	case *query.CompoundPredicate:
		return b.buildCompoundPredicateSQL(p, paramCounter)
	default:
		return "true", nil, *paramCounter
	}
}

func (b *PostgresJSONBBuilder) buildSimplePredicateSQL(predicate *query.AtomicPredicate, paramCounter *int) (string, []any, int) {
	// Check if this is a JSONB field path
	if jsonbSQL, args, ok := b.tryBuildJSONBSQL(predicate, paramCounter); ok {
		return jsonbSQL, args, *paramCounter
	}

	// Fall back to standard SQL building for non-JSONB fields
	return b.buildStandardPredicateSQL(predicate, paramCounter)
}

func (b *PostgresJSONBBuilder) buildCompoundPredicateSQL(predicate *query.CompoundPredicate, paramCounter *int) (string, []any, int) {
	if len(predicate.Predicates) == 0 {
		return "true", nil, *paramCounter
	}
	if len(predicate.Predicates) == 1 {
		return b.buildSQL(predicate.Predicates[0], paramCounter)
	}

	parts := make([]string, len(predicate.Predicates))
	var args []any

	for i, pred := range predicate.Predicates {
		sql, predArgs, _ := b.buildSQL(pred, paramCounter)
		parts[i] = fmt.Sprintf("(%s)", sql)
		args = append(args, predArgs...)
	}

	return strings.Join(parts, fmt.Sprintf(" %s ", predicate.Operator)), args, *paramCounter
}

// buildStandardPredicateSQL builds SQL for non-JSONB fields using Postgres placeholders
func (b *PostgresJSONBBuilder) buildStandardPredicateSQL(predicate *query.AtomicPredicate, paramCounter *int) (string, []any, int) {
	field := string(predicate.Field)
	mappedName, found := b.fieldMappings[field]
	if found {
		field = mappedName
	}
	switch predicate.Operator {
	case query.OpIsNull:
		return fmt.Sprintf("%s IS NULL", field), nil, *paramCounter
	case query.OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", field), nil, *paramCounter
	case query.OpIn, query.OpNotIn:
		values := predicate.Value.([]any)
		placeholders := make([]string, len(values))
		for i := range placeholders {
			*paramCounter++
			placeholders[i] = fmt.Sprintf("$%d", *paramCounter)
		}
		sql := fmt.Sprintf("%s %s (%s)", field, predicate.Operator, strings.Join(placeholders, ","))
		return sql, values, *paramCounter
	default:
		*paramCounter++
		return fmt.Sprintf("%s %s $%d", field, predicate.Operator, *paramCounter), []any{predicate.Value}, *paramCounter
	}
}

// tryBuildJSONBSQL attempts to build a JSONB query for the predicate
// Returns (sqlString, args, success)
func (b *PostgresJSONBBuilder) tryBuildJSONBSQL(predicate *query.AtomicPredicate, paramCounter *int) (string, []any, bool) {
	parts := strings.Split(string(predicate.Field), ".")

	// Check if the first part is a JSONB field
	rootField := strings.ToLower(parts[0])
	if !b.jsonbFields[rootField] {
		return "", nil, false
	}

	// Get the field type
	fieldType, exists := b.jsonbFieldTypes[rootField]
	if !exists {
		fieldType = JSONBFieldTypeArrayOfObjects // Default
	}

	// If it's just a root field with no nested path
	if len(parts) == 1 {
		mappedField := parts[0]
		if mappedName, found := b.fieldMappings[mappedField]; found {
			mappedField = mappedName
		} else {
			mappedField = rootField
		}
		sql, args := b.buildJSONBCondition(mappedField, []string{}, fieldType, predicate, paramCounter)
		return sql, args, true
	}

	// Build the JSONB path: parts[1:] contains the nested path (preserve original casing)
	path := parts[1:]
	mappedRoot := rootField
	if mappedName, found := b.fieldMappings[parts[0]]; found {
		mappedRoot = mappedName
	}
	sql, args := b.buildJSONBCondition(mappedRoot, path, fieldType, predicate, paramCounter)
	return sql, args, true
}

// buildJSONBCondition constructs a Postgres JSONB query
func (b *PostgresJSONBBuilder) buildJSONBCondition(
	rootField string,
	path []string,
	fieldType JSONBFieldType,
	predicate *query.AtomicPredicate,
	paramCounter *int,
) (string, []any) {

	switch predicate.Operator {
	case query.OpIsNull:
		return b.buildJSONBIsNull(rootField, path, fieldType), nil
	case query.OpIsNotNull:
		return b.buildJSONBIsNotNull(rootField, path, fieldType), nil
	case query.OpEqual:
		return b.buildJSONBEqual(rootField, path, fieldType, predicate.Value, paramCounter)
	case query.OpNotEqual:
		return b.buildJSONBNotEqual(rootField, path, fieldType, predicate.Value, paramCounter)
	case query.OpIn:
		return b.buildJSONBIn(rootField, path, fieldType, predicate.Value.([]any), paramCounter)
	case query.OpNotIn:
		return b.buildJSONBNotIn(rootField, path, fieldType, predicate.Value.([]any), paramCounter)
	case query.OpLess:
		return b.buildJSONBComparison(rootField, path, fieldType, "<", predicate.Value, paramCounter)
	case query.OpLessEqual:
		return b.buildJSONBComparison(rootField, path, fieldType, "<=", predicate.Value, paramCounter)
	case query.OpGreater:
		return b.buildJSONBComparison(rootField, path, fieldType, ">", predicate.Value, paramCounter)
	case query.OpGreaterEqual:
		return b.buildJSONBComparison(rootField, path, fieldType, ">=", predicate.Value, paramCounter)
	case query.OpContains:
		return b.buildJSONBContains(rootField, path, fieldType, predicate.Value, paramCounter)
	default:
		// For unsupported operators, treat as a regular field
		*paramCounter++
		return fmt.Sprintf("%s %s $%d", predicate.Field, predicate.Operator, *paramCounter), []any{predicate.Value}
	}
}

// buildJSONBIsNull builds "field->'path' IS NULL" or array search equivalent
func (b *PostgresJSONBBuilder) buildJSONBIsNull(field string, path []string, fieldType JSONBFieldType) string {
	// When there's no path, we're checking if the root field is NULL
	if len(path) == 0 {
		return fmt.Sprintf("%s IS NULL", field)
	}

	// For scalar field types with a path, use direct accessor
	if fieldType == JSONBFieldTypeScalar {
		return fmt.Sprintf("%s IS NULL", b.buildJSONBPath(field, path))
	}

	// For array types with path, check if any element's field is null
	accessor := b.buildArrayElementAccessor(path, true)
	return fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE %s IS NULL)", field, accessor)
}

// buildJSONBIsNotNull builds "field->'path' IS NOT NULL" or array search equivalent
func (b *PostgresJSONBBuilder) buildJSONBIsNotNull(field string, path []string, fieldType JSONBFieldType) string {
	// When there's no path, we're checking if the root field is NOT NULL
	if len(path) == 0 {
		return fmt.Sprintf("%s IS NOT NULL", field)
	}

	// For scalar field types with a path, use direct accessor
	if fieldType == JSONBFieldTypeScalar {
		return fmt.Sprintf("%s IS NOT NULL", b.buildJSONBPath(field, path))
	}

	// For array types with path, check if any element's field is not null
	accessor := b.buildArrayElementAccessor(path, true)
	return fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE %s IS NOT NULL)", field, accessor)
}

// buildJSONBEqual builds equality condition for JSONB fields
func (b *PostgresJSONBBuilder) buildJSONBEqual(field string, path []string, fieldType JSONBFieldType, value any, paramCounter *int) (string, []any) {
	*paramCounter++

	// For scalar fields with no nested path, use direct accessor
	if fieldType == JSONBFieldTypeScalar && len(path) == 0 {
		return fmt.Sprintf("%s = $%d", field, *paramCounter), []any{value}
	}

	if fieldType == JSONBFieldTypeMapOfArrays && len(path) > 0 {
		// Use -> to get the array, then @> to check if it contains the scalar value wrapped as a JSON array
		jsonPath := b.buildJSONBAccessor(field, path, false)
		return fmt.Sprintf("%s @> jsonb_build_array($%d::text)", jsonPath, *paramCounter), []any{value}
	}

	// For scalar fields with path (shouldn't happen but handle gracefully)
	if fieldType == JSONBFieldTypeScalar {
		jsonPath := b.buildJSONBAccessor(field, path, true)
		return fmt.Sprintf("%s = $%d", jsonPath, *paramCounter), []any{value}
	}

	// For array types, use array search
	sql := b.buildJSONBArraySearch(field, path, "=", *paramCounter, fieldType)
	return sql, []any{value}
}

// buildJSONBNotEqual builds inequality condition for JSONB fields
func (b *PostgresJSONBBuilder) buildJSONBNotEqual(field string, path []string, fieldType JSONBFieldType, value any, paramCounter *int) (string, []any) {
	*paramCounter++

	// For scalar fields with no nested path, use direct accessor
	if fieldType == JSONBFieldTypeScalar && len(path) == 0 {
		return fmt.Sprintf("%s != $%d", field, *paramCounter), []any{value}
	}

	// For scalar fields with path
	if fieldType == JSONBFieldTypeScalar {
		jsonPath := b.buildJSONBAccessor(field, path, true)
		return fmt.Sprintf("%s != $%d", jsonPath, *paramCounter), []any{value}
	}

	// For array types, use array search
	sql := b.buildJSONBArraySearch(field, path, "!=", *paramCounter, fieldType)
	return sql, []any{value}
}

// buildJSONBComparison builds comparison operators (<, >, <=, >=) for JSONB fields
func (b *PostgresJSONBBuilder) buildJSONBComparison(
	field string,
	path []string,
	fieldType JSONBFieldType,
	operator string,
	value any,
	paramCounter *int,
) (string, []any) {
	*paramCounter++

	// For scalar fields, use direct accessor
	if fieldType == JSONBFieldTypeScalar {
		var jsonPath string
		if len(path) == 0 {
			jsonPath = field
		} else {
			jsonPath = b.buildJSONBAccessor(field, path, true)
		}
		// Wrap in parentheses to ensure proper precedence for type casting
		sql := fmt.Sprintf("(%s)::numeric %s $%d::numeric", jsonPath, operator, *paramCounter)
		return sql, []any{value}
	}

	// For array types with path, build accessor without text cast and wrap in parentheses
	if len(path) > 0 {
		accessor := b.buildArrayElementAccessor(path, false)
		sql := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE (%s)::numeric %s $%d::numeric)",
			field,
			accessor,
			operator,
			*paramCounter,
		)
		return sql, []any{value}
	}

	// For array of scalars with no path
	sql := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE elem::text::numeric %s $%d::numeric)",
		field,
		operator,
		*paramCounter,
	)
	return sql, []any{value}
}

// buildJSONBIn builds "IN" operator for JSONB fields
func (b *PostgresJSONBBuilder) buildJSONBIn(field string, path []string, fieldType JSONBFieldType, values []any, paramCounter *int) (string, []any) {
	placeholders := make([]string, len(values))
	for i := range placeholders {
		*paramCounter++
		placeholders[i] = fmt.Sprintf("$%d", *paramCounter)
	}

	// For scalar fields, use direct accessor
	if fieldType == JSONBFieldTypeScalar {
		var jsonPath string
		if len(path) == 0 {
			jsonPath = field
		} else {
			jsonPath = b.buildJSONBAccessor(field, path, true)
		}
		sql := fmt.Sprintf("%s IN (%s)", jsonPath, strings.Join(placeholders, ","))
		return sql, values
	}

	// For array types, use array search
	accessor := "elem"
	if len(path) > 0 {
		accessor = b.buildArrayElementAccessor(path, true)
	} else {
		accessor = "elem::text"
	}

	sql := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE %s IN (%s))",
		field,
		accessor,
		strings.Join(placeholders, ","),
	)
	return sql, values
}

// buildJSONBNotIn builds "NOT IN" operator for JSONB fields
func (b *PostgresJSONBBuilder) buildJSONBNotIn(field string, path []string, fieldType JSONBFieldType, values []any, paramCounter *int) (string, []any) {
	placeholders := make([]string, len(values))
	for i := range placeholders {
		*paramCounter++
		placeholders[i] = fmt.Sprintf("$%d", *paramCounter)
	}

	// For scalar fields, use direct accessor
	if fieldType == JSONBFieldTypeScalar {
		var jsonPath string
		if len(path) == 0 {
			jsonPath = field
		} else {
			jsonPath = b.buildJSONBAccessor(field, path, true)
		}
		sql := fmt.Sprintf("%s NOT IN (%s)", jsonPath, strings.Join(placeholders, ","))
		return sql, values
	}

	// For array types, use EXISTS to find records with at least one element NOT IN the list
	accessor := "elem"
	if len(path) > 0 {
		accessor = b.buildArrayElementAccessor(path, true)
	} else {
		accessor = "elem::text"
	}

	sql := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE %s NOT IN (%s))",
		field,
		accessor,
		strings.Join(placeholders, ","),
	)
	return sql, values
}

// buildJSONBContains builds a JSONB contains query
func (b *PostgresJSONBBuilder) buildJSONBContains(field string, path []string, fieldType JSONBFieldType, value any, paramCounter *int) (string, []any) {
	*paramCounter++

	var jsonPath string
	if fieldType == JSONBFieldTypeScalar {
		if len(path) == 0 {
			jsonPath = field
		} else {
			jsonPath = b.buildJSONBAccessor(field, path, false)
		}
	} else {
		// For array types, use the field directly (contains operates on the array)
		jsonPath = field
	}

	sql := fmt.Sprintf("%s @> $%d", jsonPath, *paramCounter)
	return sql, []any{value}
}

// buildJSONBPath builds the Postgres JSONB path for a nested field
// Example: field->'nested'->'path'
// Preserves original casing of path segments to match JSON field names
func (b *PostgresJSONBBuilder) buildJSONBPath(field string, path []string) string {
	if len(path) == 0 {
		return field
	}

	result := field
	for _, p := range path {
		result = fmt.Sprintf("%s->'%s'", result, p)
	}
	return result
}

// buildJSONBAccessor builds the JSONB accessor with optional text cast
// textCast=true uses ->> (returns text), textCast=false uses -> (returns JSON)
// Preserves original casing of path segments to match JSON field names
func (b *PostgresJSONBBuilder) buildJSONBAccessor(field string, path []string, textCast bool) string {
	if len(path) == 0 {
		return field
	}

	result := field
	lastIdx := len(path) - 1

	// Use -> for intermediate paths
	for i := 0; i < lastIdx; i++ {
		result = fmt.Sprintf("%s->'%s'", result, path[i])
	}

	// Use ->> for final path if textCast is true
	if textCast {
		result = fmt.Sprintf("%s->>'%s'", result, path[lastIdx])
	} else {
		result = fmt.Sprintf("%s->'%s'", result, path[lastIdx])
	}

	return result
}

// buildArrayElementAccessor builds the accessor for array elements
// Used within jsonb_array_elements context
// Preserves original casing of path segments to match JSON field names
func (b *PostgresJSONBBuilder) buildArrayElementAccessor(path []string, textCast bool) string {
	if len(path) == 0 {
		if textCast {
			return "elem::text"
		}
		return "elem"
	}

	result := "elem"
	lastIdx := len(path) - 1

	// Use -> for intermediate paths
	for i := 0; i < lastIdx; i++ {
		result = fmt.Sprintf("%s->'%s'", result, path[i])
	}

	// Use ->> for final path if textCast is true
	if textCast {
		result = fmt.Sprintf("%s->>'%s'", result, path[lastIdx])
	} else {
		result = fmt.Sprintf("%s->'%s'", result, path[lastIdx])
	}

	return result
}

// buildJSONBArraySearch constructs a query to search within arrays
// For example: VPAs contains an element where cell.id = 'cell1'
func (b *PostgresJSONBBuilder) buildJSONBArraySearch(
	field string,
	path []string,
	operator string,
	paramNum int,
	fieldType JSONBFieldType,
) string {
	// For scalar array elements with no path, compare as JSONB
	if len(path) == 0 && fieldType == JSONBFieldTypeArrayOfScalars {
		return fmt.Sprintf(
			"EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE elem = to_jsonb($%d::text))",
			field,
			paramNum,
		)
	}

	accessor := b.buildArrayElementAccessor(path, true)

	return fmt.Sprintf(
		"EXISTS (SELECT 1 FROM jsonb_array_elements(%s) elem WHERE %s %s $%d)",
		field,
		accessor,
		operator,
		paramNum,
	)
}
