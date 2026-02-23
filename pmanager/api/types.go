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

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/metaform/connector-fabric-manager/common/dag"
	"github.com/metaform/connector-fabric-manager/common/model"
)

type OrchestrationState uint

const (
	OrchestrationStateInitialized OrchestrationState = 0
	OrchestrationStateRunning     OrchestrationState = 1
	OrchestrationStateCompleted   OrchestrationState = 2
	OrchestrationStateErrored     OrchestrationState = 3
)

// Orchestration is a collection of activities that are executed to allocate resources in the system. Activities are
// organized into parallel execution steps based on dependencies.
//
// As actions are completed, the orchestration system will update the Completed map.
type Orchestration struct {
	ID                string                  `json:"id"`
	CorrelationID     string                  `json:"correlationId"`
	State             OrchestrationState      `json:"state"`
	StateTimestamp    time.Time               `json:"stateTimestamp"`
	CreatedTimestamp  time.Time               `json:"createdTimestamp"`
	OrchestrationType model.OrchestrationType `json:"orchestrationType"`
	Steps             []OrchestrationStep     `json:"steps"`
	ProcessingData    map[string]any          `json:"processingData"`
	OutputData        map[string]any          `json:"outputData"`
	Completed         map[string]struct{}     `json:"completed"`
}

func (o *Orchestration) SetState(state OrchestrationState) {
	o.State = state
	o.StateTimestamp = time.Now()
}

// CanProceedToNextStep returns if the orchestration is able to proceed to the next step or must wait.
func (o *Orchestration) CanProceedToNextStep(activityId string) (bool, error) {
	step, err := o.GetStepForActivity(activityId)
	if err != nil {
		return false, err // If the step can't be found, then, we shouldn't proceed
	}

	// Check completion
	for _, activity := range step.Activities {
		if activity.ID == activityId {
			continue // Skip current activity since it is completed but not yet tracked
		}
		if _, exists := o.Completed[activity.ID]; !exists {
			return false, nil
		}
	}
	return true, nil
}

// GetStepForActivity retrieves the orchestration step containing the specified activity ID. Returns an error if not found.
func (o *Orchestration) GetStepForActivity(activityId string) (*OrchestrationStep, error) {
	for _, step := range o.Steps {
		for _, activity := range step.Activities {
			if activity.ID == activityId {
				return &step, nil
			}
		}
	}
	return nil, errors.New("step not found for activity: " + activityId)
}

// GetNextStepActivities retrieves activities from the step immediately following the one containing the specified activity.
// Returns an empty slice if the specified activity is in the last step or not found.
func (o *Orchestration) GetNextStepActivities(currentActivity string) []Activity {
	for stepIndex, step := range o.Steps {
		for _, activity := range step.Activities {
			if activity.ID == currentActivity {
				// Found the current activity, return the next step's activities
				if stepIndex+1 < len(o.Steps) {
					return o.Steps[stepIndex+1].Activities
				}
				// No next step available
				return []Activity{}
			}
		}
	}
	// Current activity not found
	return []Activity{}
}

type OrchestrationStep struct {
	Activities []Activity `json:"activities"`
}

type ActivityType string

func (at ActivityType) String() string {
	return string(at)
}

type Activity struct {
	ID            string         `json:"id"`
	Type          ActivityType   `json:"type"`
	Discriminator Discriminator  `json:"discriminator"`
	Inputs        []MappingEntry `json:"inputs"`
	DependsOn     []string       `json:"dependsOn"`
}

// ActivityMessage used to enqueue an activity for processing.
type ActivityMessage struct {
	OrchestrationID string   `json:"orchestrationID"`
	Activity        Activity `json:"activity"`
}

type MappingEntry struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// UnmarshalJSON handles deserializing a MappingEntry from a string to a source/target pair.
func (m *MappingEntry) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// If successful, use the string as both source and target
		m.Source = s
		m.Target = s
		return nil
	}

	// If string unmarshal fails, try as an object
	var objMap struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(data, &objMap); err != nil {
		return fmt.Errorf("failed to unmarshal MappingEntry: %w", err)
	}

	m.Source = objMap.Source
	m.Target = objMap.Target
	return nil
}

type OrchestrationDefinition struct {
	Type        model.OrchestrationType `json:"type"`
	TemplateRef string                  `json:"templateRef"`
	Version     int64                   `json:"version"`
	Description string                  `json:"description"`
	Active      bool                    `json:"active"`
	Schema      map[string]any          `json:"schema"`
	Activities  []Activity              `json:"activities"`
}

func (o *OrchestrationDefinition) GetID() string {
	return o.Type.String()
}

func (o *OrchestrationDefinition) GetVersion() int64 {
	return o.Version
}

func (o *OrchestrationDefinition) IncrementVersion() {
	o.Version++
}

// ActivityDefinition represents a single activity in the orchestration
type ActivityDefinition struct {
	Type         ActivityType   `json:"type"`
	Version      int64          `json:"version"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
}

func (o *ActivityDefinition) GetID() string {
	return o.Type.String()
}

func (o *ActivityDefinition) GetVersion() int64 {
	return o.Version
}

func (o *ActivityDefinition) IncrementVersion() {
	o.Version++
}

// InstantiateOrchestration creates and returns an initialized Orchestration based on the provided definition and inputs.
// It validates activity dependencies and organizes activities into parallel execution steps based on those dependencies.
func InstantiateOrchestration(
	id string,
	correlationID string,
	orchestrationType model.OrchestrationType,
	activities []Activity,
	data map[string]any) (*Orchestration, error) {

	processingData := make(map[string]any)
	for k, v := range data {
		processingData[k] = v
	}

	now := time.Now()
	orchestration := &Orchestration{
		ID:                id,
		CorrelationID:     correlationID,
		OrchestrationType: orchestrationType,
		State:             OrchestrationStateInitialized,
		StateTimestamp:    now,
		CreatedTimestamp:  now,
		Steps:             make([]OrchestrationStep, 0, len(activities)),
		ProcessingData:    processingData,
		OutputData:        make(map[string]any),
		Completed:         make(map[string]struct{}),
	}

	graph := dag.NewGraph[Activity]()
	for _, activity := range activities {
		graph.AddVertex(activity.ID, &activity)
	}
	for _, activity := range activities {
		for _, dependency := range activity.DependsOn {
			if _, exists := graph.Vertices[dependency]; !exists {
				return nil, fmt.Errorf("dependency not found: %s", dependency)
			}
			graph.AddEdge(activity.ID, dependency)
		}
	}
	sorted := graph.ParallelTopologicalSort()
	if sorted.HasCycle {
		return nil, fmt.Errorf("cycle detected in orchestration definition: %s", sorted.CyclePath)
	}

	// Reverse the iteration order because the sorted list starts with the activities that must be processed last
	levels := slices.Clone(sorted.ParallelLevels)
	slices.Reverse(levels)

	for _, level := range levels {
		step := OrchestrationStep{
			Activities: make([]Activity, 0, len(level.Vertices)),
		}
		for _, vertex := range level.Vertices {
			step.Activities = append(step.Activities, vertex.Value)
		}
		orchestration.Steps = append(orchestration.Steps, step)
	}

	return orchestration, nil
}
