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
	SortField        string      `json:"sortField"`
	FilterExpression []Criterion `json:"filterExpression"`
}
