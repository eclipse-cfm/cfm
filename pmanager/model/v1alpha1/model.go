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

package v1alpha1

import (
	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
)

const (
	DefaultActivityDiscriminator = "default"
)

type ActivityDefinitionDto struct {
	Type         string         `json:"type" validate:"required,modeltype"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}

type ActivityDto struct {
	ID        string         `json:"id" validate:"required"`
	Type      string         `json:"type" validate:"required,modeltype"`
	Inputs    []MappingEntry `json:"inputs,omitempty"`
	DependsOn []string       `json:"dependsOn,omitempty"`
}

type MappingEntry struct {
	Source string `json:"source" validate:"required"`
	Target string `json:"target" validate:"required"`
}

type OrchestrationDefinitionDto struct {
	Description string                   `json:"description,omitempty"`
	Schema      map[string]any           `json:"schema,omitempty"`
	Activities  map[string][]ActivityDto `json:"activities" validate:"required,min=1"`
	TemplateRef string                   `json:"templateRef" validate:"required"`
}

type IDResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type OrchestrationTemplate struct {
	Description string                   `json:"description,omitempty"`
	Schema      map[string]any           `json:"schema,omitempty"`
	ID          string                   `json:"id"`
	Activities  map[string][]ActivityDto `json:"activities" validate:"required,min=1"`
}

type OrchestrationEntry struct {
	ID                string                  `json:"id"`
	CorrelationID     string                  `json:"correlationId"`
	State             int                     `json:"state"`
	StateTimestamp    time.Time               `json:"stateTimestamp"`
	CreatedTimestamp  time.Time               `json:"createdTimestamp"`
	OrchestrationType model.OrchestrationType `json:"orchestrationType"`
}

type Orchestration struct {
	ID                string                  `json:"id"`
	CorrelationID     string                  `json:"correlationId"`
	State             int                     `json:"state"`
	StateTimestamp    time.Time               `json:"stateTimestamp"`
	CreatedTimestamp  time.Time               `json:"createdTimestamp"`
	OrchestrationType model.OrchestrationType `json:"orchestrationType"`
	Steps             []OrchestrationStep     `json:"steps"`
	ProcessingData    map[string]any          `json:"processingData"`
	OutputData        map[string]any          `json:"outputData"`
	Completed         map[string]struct{}     `json:"completed"`
}

type OrchestrationStep struct {
	Activities []ActivityDto `json:"activities"`
}
