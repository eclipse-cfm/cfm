/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package common

// Criterion defines the generic EDC filter expression
type Criterion struct {
	OperandLeft  any    `json:"operandLeft"`
	Operator     string `json:"operator"`
	OperandRight any    `json:"operandRight"`
}

// QuerySpec defines the generic EDC query object
type QuerySpec struct {
	Offset           int         `json:"offset"`
	Limit            int         `json:"limit"`
	SortOrder        string      `json:"sortOrder"` // ASC or DESC
	SortField        *string     `json:"sortField,omitempty"`
	FilterExpression []Criterion `json:"filterExpression"`
}

// QuerySpecOption is a functional option for QuerySpec.
type QuerySpecOption func(*QuerySpec)

// NewQuerySpec creates a QuerySpec with defaults (Offset=0, Limit=50, SortOrder=DESC, SortField=nil)
// and applies any provided options.
func NewQuerySpec(opts ...QuerySpecOption) QuerySpec {
	q := QuerySpec{
		Offset:    0,
		Limit:     50,
		SortOrder: "DESC",
		SortField: nil,
	}
	for _, opt := range opts {
		opt(&q)
	}
	return q
}

func WithOffset(offset int) QuerySpecOption {
	return func(q *QuerySpec) { q.Offset = offset }
}

func WithLimit(limit int) QuerySpecOption {
	return func(q *QuerySpec) { q.Limit = limit }
}

func WithSortOrder(order string) QuerySpecOption {
	return func(q *QuerySpec) { q.SortOrder = order }
}

func WithSortField(field string) QuerySpecOption {
	return func(q *QuerySpec) { q.SortField = &field }
}

func WithFilterCriteria(criteria ...Criterion) QuerySpecOption {
	return func(q *QuerySpec) { q.FilterExpression = append(q.FilterExpression, criteria...) }
}
