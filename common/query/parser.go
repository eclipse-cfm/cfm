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
	"fmt"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
)

// ParsePredicate parses a predicate string according to the grammar defined in Query.g4.
// This function builds the predicate structure based on the ANTLR parse tree.
// NOTE if the ANTLR grammar changes, the visiting logic in predicateBuilder will need to be updated.
// An empty or whitespace-only input is treated as a match-all predicate (no filter).
func ParsePredicate(input string) (Predicate, error) {
	if strings.TrimSpace(input) == "" {
		return &MatchAllPredicate{}, nil
	}

	is := antlr.NewInputStream(input)
	lexer := NewQueryLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, 0)
	parser := NewQueryParser(stream)

	parser.RemoveErrorListeners()
	errorListener := NewQueryErrorListener()
	parser.AddErrorListener(errorListener)

	tree := parser.Predicate()

	if errorListener.HasError() {
		return nil, fmt.Errorf("parse error: %s", errorListener.GetError())
	}

	builder := newPredicateBuilder()
	walker := antlr.NewParseTreeWalker()
	walker.Walk(builder, tree)

	return builder.GetPredicate()
}

// predicateBuilder embeds BaseQueryListener and builds Predicate objects from parse trees
type predicateBuilder struct {
	*BaseQueryListener
	stack []Predicate
	err   error
}

// newPredicateBuilder creates a new predicateBuilder
func newPredicateBuilder() *predicateBuilder {
	return &predicateBuilder{
		BaseQueryListener: &BaseQueryListener{},
		stack:             []Predicate{},
		err:               nil,
	}
}

// GetPredicate returns the top-level predicate from the parse tree
func (pb *predicateBuilder) GetPredicate() (Predicate, error) {
	if pb.err != nil {
		return nil, pb.err
	}
	if len(pb.stack) != 1 {
		return nil, fmt.Errorf("invalid parse state: expected 1 predicate, got %d", len(pb.stack))
	}
	return pb.stack[0], nil
}

// Push adds a predicate to the stack
func (pb *predicateBuilder) Push(pred Predicate) {
	pb.stack = append(pb.stack, pred)
}

// Pop removes and returns the top predicate from the stack
func (pb *predicateBuilder) Pop() Predicate {
	if len(pb.stack) == 0 {
		return nil
	}
	pred := pb.stack[len(pb.stack)-1]
	pb.stack = pb.stack[:len(pb.stack)-1]
	return pred
}

// ExitPredicate handles the top-level predicate rule, including the TRUE_KEYWORD case
func (pb *predicateBuilder) ExitPredicate(ctx *PredicateContext) {
	if pb.err != nil {
		return
	}

	// Handle the TRUE_KEYWORD case (match all predicate)
	if ctx.TRUE_KEYWORD() != nil {
		// Push a special "match all" predicate onto the stack
		pb.Push(createMatchAllPredicate())
	}
}

// createMatchAllPredicate returns a predicate that matches all objects
func createMatchAllPredicate() Predicate {
	// Return a specialized MatchAllPredicate implementation
	return &MatchAllPredicate{}
}

// ExitAtomicPredicate builds an AtomicPredicate from the context
func (pb *predicateBuilder) ExitAtomicPredicate(ctx *AtomicPredicateContext) {
	if pb.err != nil {
		return
	}

	var atomicPred *AtomicPredicate
	fieldPath := pb.getFieldPath(ctx.FieldPath())

	// Handle IS NULL / IS NOT NULL
	if ctx.NULL_KEYWORD() != nil {
		if ctx.NOT() != nil {
			// IS NOT NULL
			atomicPred = &AtomicPredicate{
				Field:    Field(fieldPath),
				Operator: OpIsNotNull,
				Value:    nil,
			}
		} else {
			// IS NULL
			atomicPred = &AtomicPredicate{
				Field:    Field(fieldPath),
				Operator: OpIsNull,
				Value:    nil,
			}
		}
	} else if ctx.IN() != nil {
		// Handle IN or NOT IN
		valueList := pb.getValueList(ctx.ValueList())
		if pb.err != nil {
			return
		}

		if ctx.NOT() != nil {
			atomicPred = &AtomicPredicate{
				Field:    Field(fieldPath),
				Operator: OpNotIn,
				Value:    valueList,
			}
		} else {
			atomicPred = &AtomicPredicate{
				Field:    Field(fieldPath),
				Operator: OpIn,
				Value:    valueList,
			}
		}
	} else if ctx.ComparisonOperator() != nil {
		// Handle comparison operators
		op := pb.parseOperator(ctx.ComparisonOperator().GetText())
		value := pb.getValue(ctx.Value())
		if pb.err != nil {
			return
		}

		atomicPred = &AtomicPredicate{
			Field:    Field(fieldPath),
			Operator: op,
			Value:    value,
		}
	}

	if atomicPred != nil {
		pb.Push(atomicPred)
	}
}

// ExitCompoundPredicate builds a CompoundPredicate from the context
func (pb *predicateBuilder) ExitCompoundPredicate(ctx *CompoundPredicateContext) {
	if pb.err != nil {
		return
	}

	// If this is a parenthesized compound predicate with inner predicate, skip
	if ctx.CompoundPredicate() != nil {
		return
	}

	numAtomics := len(ctx.AllAtomicPredicate())

	if numAtomics <= 1 {
		// Single atomic predicate - already on stack
		return
	}

	// Multiple atomic predicates - pop them in reverse order
	predicates := make([]Predicate, numAtomics)
	for i := numAtomics - 1; i >= 0; i-- {
		predicates[i] = pb.Pop()
	}

	// Determine operator by checking for AND/OR tokens
	var operator string
	allANDs := ctx.AllAND()
	allORs := ctx.AllOR()

	if len(allANDs) > 0 {
		operator = "AND"
	} else if len(allORs) > 0 {
		operator = "OR"
	} else {
		pb.err = fmt.Errorf("no operator found in compound predicate")
		return
	}

	compoundPred := &CompoundPredicate{
		Operator:   operator,
		Predicates: predicates,
	}

	pb.Push(compoundPred)
}

// getFieldPath extracts the field path from context
func (pb *predicateBuilder) getFieldPath(ctx IFieldPathContext) string {
	if ctx == nil {
		return ""
	}

	allIdents := ctx.AllIDENTIFIER()
	parts := make([]string, len(allIdents))

	for i, ident := range allIdents {
		parts[i] = ident.GetText()
	}

	return strings.Join(parts, ".")
}

// getValue converts Value context to Go value
func (pb *predicateBuilder) getValue(ctx IValueContext) any {
	if ctx == nil {
		return nil
	}

	if ctx.StringValue() != nil {
		return pb.parseStringValue(ctx.StringValue().GetText())
	}
	if ctx.NumericValue() != nil {
		return pb.parseNumericValue(ctx.NumericValue().GetText())
	}
	if ctx.BooleanValue() != nil {
		return pb.parseBooleanValue(ctx.BooleanValue().GetText())
	}
	if ctx.NULL_KEYWORD() != nil {
		return nil
	}

	pb.err = fmt.Errorf("unknown value type")
	return nil
}

// parseStringValue removes quotes and unescapes
func (pb *predicateBuilder) parseStringValue(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
	}
	s = strings.ReplaceAll(s, "''", "'")
	return s
}

// parseNumericValue converts string to int64 or float64
func (pb *predicateBuilder) parseNumericValue(s string) any {
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// parseBooleanValue converts string to bool
func (pb *predicateBuilder) parseBooleanValue(s string) bool {
	return strings.ToUpper(s) == "TRUE"
}

// parseOperator converts operator string to the Operator type
func (pb *predicateBuilder) parseOperator(opStr string) Operator {
	opUpper := strings.ToUpper(opStr)
	switch opUpper {
	case "=":
		return OpEqual
	case "!=":
		return OpNotEqual
	case ">":
		return OpGreater
	case ">=":
		return OpGreaterEqual
	case "<":
		return OpLess
	case "<=":
		return OpLessEqual
	case "LIKE":
		return OpLike
	case "NOT LIKE":
		return OpNotLike
	case "CONTAINS":
		return OpContains
	case "STARTS_WITH":
		return OpStartsWith
	case "ENDS_WITH":
		return OpEndsWith
	default:
		return Operator(opUpper)
	}
}

// getValueList converts ValueList context to slice of values
func (pb *predicateBuilder) getValueList(ctx IValueListContext) []any {
	if ctx == nil {
		return []any{}
	}

	allValues := ctx.AllValue()
	values := make([]any, len(allValues))

	for i, valCtx := range allValues {
		values[i] = pb.getValue(valCtx)
		if pb.err != nil {
			return nil
		}
	}

	return values
}

// QueryErrorListener captures ANTLR parse errors
type QueryErrorListener struct {
	*antlr.DefaultErrorListener
	errors []string
}

// NewQueryErrorListener creates a new error listener
func NewQueryErrorListener() *QueryErrorListener {
	return &QueryErrorListener{
		errors: []string{},
	}
}

// SyntaxError captures syntax errors
func (el *QueryErrorListener) SyntaxError(_ antlr.Recognizer, _ any, line, column int, msg string, _ antlr.RecognitionException) {
	errorMsg := fmt.Sprintf("line %d:%d %s", line, column, msg)
	el.errors = append(el.errors, errorMsg)
}

// HasError returns true if errors were encountered
func (el *QueryErrorListener) HasError() bool {
	return len(el.errors) > 0
}

// GetError returns the first error message
func (el *QueryErrorListener) GetError() string {
	if len(el.errors) > 0 {
		return el.errors[0]
	}
	return ""
}

// GetAllErrors returns all error messages
func (el *QueryErrorListener) GetAllErrors() []string {
	return el.errors
}
