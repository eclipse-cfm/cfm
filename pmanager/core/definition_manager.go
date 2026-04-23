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

package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
)

type definitionManager struct {
	trxContext         store.TransactionContext
	store              api.DefinitionStore
	orchestrationStore store.EntityStore[*api.OrchestrationEntry]
}

func (d definitionManager) CreateOrchestrationDefinition(ctx context.Context, definition *api.OrchestrationDefinition) (*api.OrchestrationDefinition, error) {
	return store.Trx[api.OrchestrationDefinition](d.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.OrchestrationDefinition, error) {
		var missingErrors []error

		activityDefinitions := make(map[api.ActivityType]*api.ActivityDefinition)

		// Verify that all referenced activities exist
		for _, activity := range definition.Activities {
			activityDefinition, err := d.store.FindActivityDefinition(ctx, activity.Type)
			if err != nil {
				if errors.Is(err, types.ErrNotFound) {
					missingErrors = append(missingErrors, types.NewClientError("activity type '%s' not found", activity.Type))
					continue
				}
				return nil, err
			}
			activityDefinitions[activity.Type] = activityDefinition
		}

		if len(missingErrors) > 0 {
			return nil, errors.Join(missingErrors...)
		}
		// Verify schema compatibility between dependent activities for deploy orchestrations
		if definition.Type == model.VPADeployType {
			if err := validateActivitySchema(definition.Activities, activityDefinitions); err != nil {
				return nil, err
			}
		}

		persisted, err := d.store.StoreOrchestrationDefinition(ctx, definition)
		if err != nil {
			return nil, err
		}
		return persisted, nil
	})
}

func (d definitionManager) DeleteOrchestrationDefinition(ctx context.Context, templateRef string) error {

	return d.trxContext.Execute(ctx, func(ctx context.Context) error {

		templateRefPredicate := query.Eq("templateRef", templateRef)
		defs, err := collection.CollectAll(d.store.FindOrchestrationDefinitionsByPredicate(ctx, templateRefPredicate))

		if err != nil {
			return types.NewRecoverableWrappedError(err, "failed to check orchestration definition for template-ref %s", templateRef)
		}
		if len(defs) == 0 {
			return types.ErrNotFound
		}
		for _, def := range defs {

			// check if any orchestration-definition has ongoing orchestrations
			orchestrationEntry, err := d.orchestrationStore.FindFirstByPredicate(ctx, query.Eq("DefinitionID", def.GetID()))
			if err != nil && !errors.Is(err, types.ErrNotFound) {
				return types.NewRecoverableWrappedError(err, "failed to delete orchestration definition %s, because checking for ongoing orchestrations failed", def.GetID())
			}
			// todo: should we allow deleting orch-defs that have _completed_/_errored_ orchestrations?
			if orchestrationEntry != nil {
				return types.NewClientError("Cannot delete orchestration definition %s because it has ongoing orchestrations", def.GetID())
			}

			// execute deletion
			deleted, err := d.store.DeleteOrchestrationDefinition(ctx, def.Type)
			if err != nil {
				return types.NewRecoverableWrappedError(err, "failed to delete orchestration definition for template-ref %s", templateRef)
			}
			if !deleted {
				return types.NewClientError("unable to delete orchestration definition template-ref %s", templateRef)
			}
		}
		return nil
	})
}

func (d definitionManager) GetOrchestrationDefinitionsByTemplate(ctx context.Context, templateRef string) ([]api.OrchestrationDefinition, error) {
	return d.QueryOrchestrationDefinitions(ctx, query.Eq("templateRef", templateRef))
}

func (d definitionManager) GetOrchestrationDefinitions(ctx context.Context) ([]api.OrchestrationDefinition, error) {
	var result []api.OrchestrationDefinition
	err := d.trxContext.Execute(ctx, func(ctx context.Context) error {
		definitions, err := d.store.ListOrchestrationDefinitions(ctx)
		if err != nil {
			return err
		}
		result = definitions
		return nil
	})
	return result, err
}

func (d definitionManager) QueryOrchestrationDefinitions(ctx context.Context, predicate query.Predicate) ([]api.OrchestrationDefinition, error) {
	var result []api.OrchestrationDefinition

	err := d.trxContext.Execute(ctx, func(ctx context.Context) error {
		definitions, err := collection.CollectAll(d.store.FindOrchestrationDefinitionsByPredicate(ctx, predicate))
		if err != nil {
			return err
		}
		result = definitions
		return nil
	})

	return result, err
}

func (d definitionManager) CreateActivityDefinition(ctx context.Context, definition *api.ActivityDefinition) (*api.ActivityDefinition, error) {
	return store.Trx[api.ActivityDefinition](d.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.ActivityDefinition, error) {
		definition, err := d.store.StoreActivityDefinition(ctx, definition)
		if err != nil {
			return nil, err
		}
		return definition, nil
	})
}

func (d definitionManager) DeleteActivityDefinition(ctx context.Context, atype api.ActivityType) error {
	return d.trxContext.Execute(ctx, func(ctx context.Context) error {
		exists, err := d.store.ExistsActivityDefinition(ctx, atype)
		if err != nil {
			return types.NewRecoverableWrappedError(err, "failed to check activity definition for type %s", atype)
		}
		if !exists {
			return types.ErrNotFound
		}
		referenced, err := d.store.ActivityDefinitionReferences(ctx, atype)

		if err != nil {
			return types.NewRecoverableWrappedError(err, "failed to check activity definition references for type %s", atype)
		}
		if len(referenced) > 0 {
			return types.NewClientError("activity type '%s' is referenced by an orchestration definition: %s", atype, strings.Join(referenced, ", "))
		}

		deleted, err := d.store.DeleteActivityDefinition(ctx, atype)
		if err != nil {
			return types.NewRecoverableWrappedError(err, "failed to check activity definition references for type %s", atype)
		}
		if !deleted {
			return types.NewClientError("unable to delete activity definition type %s", atype)
		}
		return nil
	})
}

func (d definitionManager) GetActivityDefinitions(ctx context.Context) ([]api.ActivityDefinition, error) {
	var result []api.ActivityDefinition
	err := d.trxContext.Execute(ctx, func(ctx context.Context) error {
		definitions, err := d.store.ListActivityDefinitions(ctx)
		if err != nil {
			return err
		}
		result = definitions
		return nil
	})
	return result, err
}

func validateActivitySchema(activities []api.Activity, activityDefinitions map[api.ActivityType]*api.ActivityDefinition) error {
	activityMapById := make(map[string]api.Activity)

	for _, activity := range activities {
		activityMapById[activity.ID] = activity
	}

	var validationErrors []error

	for _, activity := range activities {

		mergedDependantOutputProperties := make(map[string]any)

		//The following 3 fields are always injected as part of the orchestration manifest. In a future this could be modified and maybe derived from the orchestration schema
		mergedDependantOutputProperties[model.ParticipantIdentifier] = map[string]any{"type": "string"}
		mergedDependantOutputProperties[model.VPAData] = map[string]any{"type": "array"}
		mergedDependantOutputProperties[model.CredentialData] = map[string]any{"type": "array"}

		hasDuplicateFieldError := false

		for _, dependencyID := range activity.DependsOn {
			dependantActivity := activityMapById[dependencyID]
			dependantActivityDefinition := activityDefinitions[dependantActivity.Type]
			dependantOutputProperties, ok := dependantActivityDefinition.OutputSchema["properties"].(map[string]any)
			if !ok {
				continue
			}
			for field := range dependantOutputProperties {
				if _, exists := mergedDependantOutputProperties[field]; exists {
					validationErrors = append(validationErrors, types.NewValidationError(activity.ID, fmt.Sprintf("field %s is duplicated in multiple dependent activities", field)))
					hasDuplicateFieldError = true
				} else {
					mergedDependantOutputProperties[field] = dependantOutputProperties[field]
				}
			}
		}
		if hasDuplicateFieldError {
			continue
		}

		requiredFields, _ := activityDefinitions[activity.Type].InputSchema["required"].([]any)
		inputProperties, _ := activityDefinitions[activity.Type].InputSchema["properties"].(map[string]any)

		for _, required := range requiredFields {
			field, ok := required.(string)
			if !ok {
				continue
			}

			if _, exists := mergedDependantOutputProperties[field]; !exists {
				validationErrors = append(validationErrors, types.NewValidationError(activity.ID, fmt.Sprintf("required input field %s is not provided by dependent activities as output", field)))
			}
		}

		for field := range inputProperties {
			dependantField, exists := mergedDependantOutputProperties[field]
			if !exists {
				continue
			}
			inputFieldType, _ := inputProperties[field].(map[string]any)["type"].(string)
			dependantFieldType, _ := dependantField.(map[string]any)["type"].(string)

			if inputFieldType != "" && dependantFieldType != "" && inputFieldType != dependantFieldType {
				validationErrors = append(validationErrors, types.NewValidationError(activity.ID, fmt.Sprintf("type of input field %s differs from dependent activities output", field)))
			}

		}

	}

	return errors.Join(validationErrors...)
}
