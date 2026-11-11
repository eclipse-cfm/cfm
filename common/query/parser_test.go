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

package query

import (
	"testing"
)

// TestParseAtomicPredicate tests parsing of basic atomic predicates
func TestParseAtomicPredicate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "Equality",
			input:     "Name = 'Alice'",
			expectErr: false,
		},
		{
			name:      "Not Equal",
			input:     "Status != 'inactive'",
			expectErr: false,
		},
		{
			name:      "Greater than",
			input:     "Age > 30",
			expectErr: false,
		},
		{
			name:      "Greater or equal",
			input:     "Age >= 25",
			expectErr: false,
		},
		{
			name:      "Less than",
			input:     "Age < 65",
			expectErr: false,
		},
		{
			name:      "Less or equal",
			input:     "Score <= 95.5",
			expectErr: false,
		},
		{
			name:      "IS NULL",
			input:     "DeletedAt IS NULL",
			expectErr: false,
		},
		{
			name:      "IS NOT NULL",
			input:     "UpdatedAt IS NOT NULL",
			expectErr: false,
		},
		{
			name:      "IN operator",
			input:     "Status IN ('active', 'pending')",
			expectErr: false,
		},
		{
			name:      "NOT IN operator",
			input:     "Status NOT IN ('inactive', 'deleted')",
			expectErr: false,
		},
		{
			name:      "LIKE operator",
			input:     "Name LIKE 'A%'",
			expectErr: false,
		},
		{
			name:      "CONTAINS",
			input:     "Email CONTAINS '@example.com'",
			expectErr: false,
		},
		{
			name:      "STARTS_WITH",
			input:     "Name STARTS_WITH 'J'",
			expectErr: false,
		},
		{
			name:      "ENDS_WITH",
			input:     "Email ENDS_WITH '.com'",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := ParsePredicate(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("ParsePredicate() error = %v, expectErr = %v", err, tt.expectErr)
				return
			}

			if err == nil && pred == nil {
				t.Errorf("ParsePredicate() returned nil predicate")
				return
			}
		})
	}
}

// TestParseCompoundPredicate tests parsing of compound predicates
func TestParseCompoundPredicate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "Simple AND",
			input:     "Name = 'Alice' AND Age > 25",
			expectErr: false,
		},
		{
			name:      "Simple OR",
			input:     "Status = 'active' OR Status = 'pending'",
			expectErr: false,
		},
		{
			name:      "Parenthesized AND",
			input:     "(Name = 'Alice' AND Age > 25)",
			expectErr: false,
		},
		{
			name:      "Multiple AND",
			input:     "Name = 'Alice' AND Age > 25 AND Status = 'active'",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := ParsePredicate(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("ParsePredicate() error = %v, expectErr = %v", err, tt.expectErr)
				return
			}

			if err == nil && pred == nil {
				t.Errorf("ParsePredicate() returned nil predicate")
			}
		})
	}
}

// TestParse_MatchingBehavior tests that parsed predicates work correctly
func TestParse_MatchingBehavior(t *testing.T) {
	entity := TestEntity{
		ID:       "123",
		Name:     "Alice",
		Age:      30,
		Score:    95.5,
		Active:   true,
		Email:    "alice@example.com",
		Category: "Premium",
		Count:    5,
	}

	tests := []struct {
		name          string
		input         string
		expectedMatch bool
	}{
		{
			name:          "Equality match",
			input:         "Name = 'Alice'",
			expectedMatch: true,
		},
		{
			name:          "Equality no match",
			input:         "Name = 'Bob'",
			expectedMatch: false,
		},
		{
			name:          "Greater than match",
			input:         "Age > 25",
			expectedMatch: true,
		},
		{
			name:          "AND both true",
			input:         "Name = 'Alice' AND Age > 25",
			expectedMatch: true,
		},
		{
			name:          "AND one false",
			input:         "Name = 'Alice' AND Age < 25",
			expectedMatch: false,
		},
		{
			name:          "OR first true",
			input:         "Name = 'Alice' OR Age < 25",
			expectedMatch: true,
		},
		{
			name:          "OR both false",
			input:         "Name = 'Bob' OR Age < 25",
			expectedMatch: false,
		},
		{
			name:          "IN match",
			input:         "Category IN ('Premium', 'Gold')",
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := ParsePredicate(tt.input)
			if err != nil {
				t.Fatalf("ParsePredicate() error = %v", err)
			}

			result := pred.Matches(entity, nil)
			if result != tt.expectedMatch {
				t.Errorf("Matches() = %v, want %v", result, tt.expectedMatch)
			}
		})
	}
}

// TestParse_QuotedStrings tests parsing of strings with special characters
func TestParse_QuotedStrings(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Simple string",
			input:       "Name = 'Alice'",
			expectError: false,
		},
		{
			name:        "String with special characters",
			input:       "Email = 'alice@example.com'",
			expectError: false,
		},
		{
			name:        "String with hyphen",
			input:       "ID = 'test-123'",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := ParsePredicate(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("ParsePredicate() error = %v, expectError = %v", err, tt.expectError)
				return
			}
			if err == nil && pred == nil {
				t.Errorf("ParsePredicate() returned nil predicate")
			}
		})
	}
}

// TestParse_CaseInsensitiveKeywords tests that keywords work in both uppercase and lowercase
func TestParse_CaseInsensitiveKeywords(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "DeletedAt is null - lowercase",
			input:     "DeletedAt is null",
			expectErr: false,
		},
		{
			name:      "DeletedAt IS NULL - uppercase",
			input:     "DeletedAt IS NULL",
			expectErr: false,
		},
		{
			name:      "UpdatedAt is not null - lowercase",
			input:     "UpdatedAt is not null",
			expectErr: false,
		},
		{
			name:      "UpdatedAt IS NOT NULL - uppercase",
			input:     "UpdatedAt IS NOT NULL",
			expectErr: false,
		},
		{
			name:      "Status in list - lowercase",
			input:     "Status in ('active', 'pending')",
			expectErr: false,
		},
		{
			name:      "Status IN list - uppercase",
			input:     "Status IN ('active', 'pending')",
			expectErr: false,
		},
		{
			name:      "Name and condition - lowercase",
			input:     "Name = 'Alice' and Age > 25",
			expectErr: false,
		},
		{
			name:      "Name AND condition - uppercase",
			input:     "Name = 'Alice' AND Age > 25",
			expectErr: false,
		},
		{
			name:      "Status or condition - lowercase",
			input:     "Status = 'active' or Status = 'pending'",
			expectErr: false,
		},
		{
			name:      "Status OR condition - uppercase",
			input:     "Status = 'active' OR Status = 'pending'",
			expectErr: false,
		},
		{
			name:      "Mixed case keywords",
			input:     "DeletedAt is null AND Status in ('active', 'pending')",
			expectErr: false,
		},
		{
			name:      "like - lowercase",
			input:     "Email like '%@example.com'",
			expectErr: false,
		},
		{
			name:      "LIKE - uppercase",
			input:     "Email LIKE '%@example.com'",
			expectErr: false,
		},
		{
			name:      "contains - lowercase",
			input:     "Email contains '@'",
			expectErr: false,
		},
		{
			name:      "CONTAINS - uppercase",
			input:     "Email CONTAINS '@'",
			expectErr: false,
		},
		{
			name:      "starts_with - lowercase",
			input:     "Name starts_with 'A'",
			expectErr: false,
		},
		{
			name:      "STARTS_WITH - uppercase",
			input:     "Name STARTS_WITH 'A'",
			expectErr: false,
		},
		{
			name:      "ends_with - lowercase",
			input:     "Email ends_with '.com'",
			expectErr: false,
		},
		{
			name:      "ENDS_WITH - uppercase",
			input:     "Email ENDS_WITH '.com'",
			expectErr: false,
		},
		{
			name:      "not in - lowercase",
			input:     "Status not in ('inactive', 'deleted')",
			expectErr: false,
		},
		{
			name:      "NOT IN - uppercase",
			input:     "Status NOT IN ('inactive', 'deleted')",
			expectErr: false,
		},
		{
			name:      "true - lowercase",
			input:     "Active = true",
			expectErr: false,
		},
		{
			name:      "TRUE - uppercase",
			input:     "Active = TRUE",
			expectErr: false,
		},
		{
			name:      "false - lowercase",
			input:     "Active = false",
			expectErr: false,
		},
		{
			name:      "FALSE - uppercase",
			input:     "Active = FALSE",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := ParsePredicate(tt.input)
			if (err != nil) != tt.expectErr {
				t.Errorf("ParsePredicate() error = %v, expectErr = %v", err, tt.expectErr)
				return
			}

			if err == nil && pred == nil {
				t.Errorf("ParsePredicate() returned nil predicate")
				return
			}
		})
	}
}

// TestParse_EdgeCases tests edge cases and error handling
func TestParse_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectError    bool
		expectMatchAll bool
	}{
		{
			name:           "Empty string returns match-all",
			input:          "",
			expectError:    false,
			expectMatchAll: true,
		},
		{
			name:           "Whitespace only returns match-all",
			input:          "   ",
			expectError:    false,
			expectMatchAll: true,
		},
		{
			name:           "Valid AND",
			input:          "Name = 'Alice' AND Age > 25",
			expectError:    false,
			expectMatchAll: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := ParsePredicate(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("ParsePredicate() error = %v, expectError = %v", err, tt.expectError)
				return
			}
			if tt.expectMatchAll {
				if _, ok := pred.(*MatchAllPredicate); !ok {
					t.Errorf("expected *MatchAllPredicate, got %T", pred)
				}
			}
		})
	}
}

// TestParseTrueKeyword tests parsing of the true keyword as a match-all predicate
func TestParseTrueKeyword(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectError    bool
		expectMatchAll bool
	}{
		{
			name:           "true (lowercase)",
			input:          "true",
			expectError:    false,
			expectMatchAll: true,
		},
		{
			name:           "TRUE (uppercase)",
			input:          "TRUE",
			expectError:    false,
			expectMatchAll: true,
		},
		{
			name:           "true with whitespace",
			input:          "  true  ",
			expectError:    false,
			expectMatchAll: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred, err := ParsePredicate(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if pred == nil {
				t.Errorf("expected predicate, got nil")
				return
			}

			// Verify it's a MatchAllPredicate
			matchAllPred, ok := pred.(*MatchAllPredicate)
			if !ok {
				t.Errorf("expected *MatchAllPredicate, got %T", pred)
				return
			}

			// Verify it matches everything
			testObj := struct {
				Name  string
				Value int
			}{
				Name:  "test",
				Value: 42,
			}

			if !matchAllPred.Matches(testObj, &DefaultFieldMatcher{}) {
				t.Errorf("expected MatchAllPredicate to match all objects")
			}
		})
	}
}

// TestTrueKeywordMaturesAllObjects verifies the true keyword matches any object
func TestTrueKeywordMattersAllObjects(t *testing.T) {
	pred, err := ParsePredicate("true")
	if err != nil {
		t.Fatalf("failed to parse 'true': %v", err)
	}

	testCases := []any{
		struct{ Name string }{Name: "test"},
		struct{ Value int }{Value: 123},
		struct {
			Name  string
			Value int
			Tags  []string
		}{Name: "complex", Value: 456, Tags: []string{"a", "b"}},
		"simple string",
		42,
		nil,
	}

	matcher := &DefaultFieldMatcher{}
	for i, obj := range testCases {
		if !pred.Matches(obj, matcher) {
			t.Errorf("test case %d: expected true predicate to match object %v", i, obj)
		}
	}
}

// BenchmarkParsePredicate benchmarks ANTLR parsing performance
func BenchmarkParsePredicate(b *testing.B) {
	input := "Name = 'John' AND Age > 30 OR Status IN ('active', 'pending')"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParsePredicate(input)
	}
}
