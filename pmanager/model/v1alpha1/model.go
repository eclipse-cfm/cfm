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

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/types"
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
	ID        string   `json:"id" validate:"required"`
	Type      string   `json:"type" validate:"required,modeltype"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

type OrchestrationDefinitionDto struct {
	Description string                   `json:"description,omitempty"`
	Schema      map[string]any           `json:"schema,omitempty"`
	Activities  map[string][]ActivityDto `json:"activities" validate:"required,min=1,dive,dive"`
	TemplateRef string                   `json:"templateRef" validate:"required"`
	// CompensationType is the orchestration type that compensates this definition when one of its orchestrations fails
	CompensationType string `json:"compensationType,omitempty"`
}

type IDResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type OrchestrationTemplate struct {
	Description string                   `json:"description,omitempty"`
	Schema      map[string]any           `json:"schema,omitempty"`
	ID          string                   `json:"id"`
	Activities  map[string][]ActivityDto `json:"activities" validate:"required,min=1,dive,dive"`
	// Compensation maps an orchestration type in Activities to the orchestration type in Activities that is started when
	// an orchestration of the first type fails. An orchestration type without an entry is not compensated.
	Compensation map[string]string `json:"compensation,omitempty"`
}

// ValidateCompensation checks the Compensation map of the template. Every key and every value must be an orchestration
// type in Activities, an orchestration type must not compensate itself, and following the map from any orchestration
// type must not lead back to it, because a failed compensation would otherwise start the orchestration it compensates.
func (t *OrchestrationTemplate) ValidateCompensation() error {
	for orchType, compensationType := range t.Compensation {
		if _, ok := t.Activities[orchType]; !ok {
			return types.NewValidationError("compensation."+orchType, "orchestration type is not defined in activities")
		}
		if _, ok := t.Activities[compensationType]; !ok {
			return types.NewValidationError("compensation."+orchType, "compensation type ["+compensationType+"] is not defined in activities")
		}
		if compensationType == orchType {
			return types.NewValidationError("compensation."+orchType, "an orchestration type cannot compensate itself")
		}
		visited := map[string]bool{orchType: true}
		for next, ok := compensationType, true; ok; next, ok = t.Compensation[next] {
			if visited[next] {
				return types.NewValidationError("compensation."+orchType, "compensation types form a cycle")
			}
			visited[next] = true
		}
	}
	return nil
}

type OrchestrationEntry struct {
	ID                string                  `json:"id"`
	CorrelationID     string                  `json:"correlationId"`
	State             int                     `json:"state"`
	StateTimestamp    time.Time               `json:"stateTimestamp"`
	CreatedTimestamp  time.Time               `json:"createdTimestamp"`
	OrchestrationType model.OrchestrationType `json:"orchestrationType"`
	DefinitionID      string                  `json:"definitionId"`
}

type Orchestration struct {
	ID                string                  `json:"id"`
	CorrelationID     string                  `json:"correlationId"`
	DefinitionID      string                  `json:"definitionId"`
	State             int                     `json:"state"`
	StateTimestamp    time.Time               `json:"stateTimestamp"`
	CreatedTimestamp  time.Time               `json:"createdTimestamp"`
	OrchestrationType model.OrchestrationType `json:"orchestrationType"`
	Steps             []OrchestrationStep     `json:"steps"`
	ProcessingData    map[string]any          `json:"processingData"`
	OutputData        map[string]any          `json:"outputData"`
	Completed         map[string]struct{}     `json:"completed"`
	// Error is the error that put the orchestration into the errored state, if any.
	Error string `json:"error,omitempty"`
}

type OrchestrationStep struct {
	Activities []ActivityDto `json:"activities"`
}
