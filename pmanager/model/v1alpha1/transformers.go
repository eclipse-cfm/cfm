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
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
)

func ToActivityDefinitionDto(definition *api.ActivityDefinition) *ActivityDefinitionDto {
	if definition == nil {
		return &ActivityDefinitionDto{}
	}
	return &ActivityDefinitionDto{
		Type:         string(definition.Type),
		Description:  definition.Description,
		InputSchema:  definition.InputSchema,
		OutputSchema: definition.OutputSchema,
	}
}

func ToActivityDefinition(definition *ActivityDefinitionDto) *api.ActivityDefinition {
	if definition == nil {
		return &api.ActivityDefinition{}
	}
	return &api.ActivityDefinition{
		Type:         api.ActivityType(definition.Type),
		Description:  definition.Description,
		InputSchema:  definition.InputSchema,
		OutputSchema: definition.OutputSchema,
	}
}

// ToOrchestrationDefinition converts an OrchestrationTemplate to one or several api.OrchestrationDefinition structures
func ToOrchestrationDefinition(template *OrchestrationTemplate) []*api.OrchestrationDefinition {
	if template == nil {
		return []*api.OrchestrationDefinition{}
	}

	// determine number of orchestrations
	convertedOrchestrationDefs := make([]*api.OrchestrationDefinition, 0)

	if template.Activities != nil && len(template.Activities) == 0 {
		convertedOrchestrationDefs = append(convertedOrchestrationDefs, &api.OrchestrationDefinition{
			Description: template.Description,
			Active:      true,
			Schema:      template.Schema,
			Activities:  make([]api.Activity, 0),
		})
	}

	// generate one api.OrchestrationDefinition for each entry
	for orchType, activities := range template.Activities {

		// convert activity DTOs to activities
		convertedActivities := make([]api.Activity, len(activities))
		for i, activityDto := range activities {
			activity := api.Activity{
				ID:            activityDto.ID,
				Type:          api.ActivityType(activityDto.Type),
				Discriminator: api.Discriminator(orchType),
				Inputs:        ToAPIMappingEntries(activityDto.Inputs),
				DependsOn:     activityDto.DependsOn,
			}
			convertedActivities[i] = activity
		}

		// create orchestration template
		orchestrationDef := api.OrchestrationDefinition{
			Type:        model.OrchestrationType(orchType),
			Description: template.Description,
			Active:      true,
			Schema:      template.Schema,
			Activities:  convertedActivities,
		}

		convertedOrchestrationDefs = append(convertedOrchestrationDefs, &orchestrationDef)
	}
	return convertedOrchestrationDefs
}

func ToOrchestrationDefinitionDto(definition *api.OrchestrationDefinition) *OrchestrationDefinitionDto {
	if definition == nil {
		return &OrchestrationDefinitionDto{}
	}

	// for each individual activity discriminator, create one map entry
	convertedActivities := make(map[string][]ActivityDto)

	for _, activity := range definition.Activities {
		convertedActivities[activity.Discriminator.String()] = append(convertedActivities[activity.Discriminator.String()], ActivityDto{
			ID:        activity.ID,
			Type:      string(activity.Type),
			Inputs:    ToMappingEntries(activity.Inputs),
			DependsOn: activity.DependsOn,
		})
	}

	return &OrchestrationDefinitionDto{
		Description: definition.Description,
		Schema:      definition.Schema,
		Activities:  convertedActivities,
	}
}

func ToAPIMappingEntries(entries []MappingEntry) []api.MappingEntry {
	apiEntries := make([]api.MappingEntry, len(entries))
	for i, entry := range entries {
		apiEntries[i] = api.MappingEntry{
			Source: entry.Source,
			Target: entry.Target,
		}
	}
	return apiEntries
}

func ToMappingEntries(entries []api.MappingEntry) []MappingEntry {
	apiEntries := make([]MappingEntry, len(entries))
	for i, entry := range entries {
		apiEntries[i] = MappingEntry{
			Source: entry.Source,
			Target: entry.Target,
		}
	}
	return apiEntries
}

func ToOrchestrationEntry(entry *api.OrchestrationEntry) OrchestrationEntry {
	return OrchestrationEntry{
		ID:                entry.ID,
		CorrelationID:     entry.CorrelationID,
		State:             int(entry.State),
		StateTimestamp:    entry.StateTimestamp,
		CreatedTimestamp:  entry.CreatedTimestamp,
		OrchestrationType: entry.OrchestrationType,
	}
}

func ToOrchestration(orchestration *api.Orchestration) Orchestration {
	return Orchestration{
		ID:                orchestration.ID,
		CorrelationID:     orchestration.CorrelationID,
		State:             int(orchestration.State),
		StateTimestamp:    orchestration.StateTimestamp,
		CreatedTimestamp:  orchestration.CreatedTimestamp,
		OrchestrationType: orchestration.OrchestrationType,
		ProcessingData:    orchestration.ProcessingData,
		Steps:             toSteps(orchestration.Steps),
		OutputData:        orchestration.OutputData,
		Completed:         orchestration.Completed,
	}
}

func toSteps(steps []api.OrchestrationStep) []OrchestrationStep {
	result := make([]OrchestrationStep, len(steps))
	for i, step := range steps {
		result[i] = OrchestrationStep{
			Activities: toActivities(step.Activities),
		}
	}
	return result
}

func toActivities(activities []api.Activity) []ActivityDto {
	result := make([]ActivityDto, len(activities))
	for i, activity := range activities {
		result[i] = ActivityDto{
			ID:        activity.ID,
			Type:      string(activity.Type),
			Inputs:    toMappingEntries(activity.Inputs),
			DependsOn: activity.DependsOn,
		}
	}
	return result
}

func toMappingEntries(entries []api.MappingEntry) []MappingEntry {
	result := make([]MappingEntry, len(entries))
	for i, entry := range entries {
		result[i] = MappingEntry{
			Source: entry.Source,
			Target: entry.Target,
		}
	}
	return result
}
