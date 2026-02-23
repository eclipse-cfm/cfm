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
	"strings"

	"github.com/metaform/connector-fabric-manager/common/collection"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
)

type definitionManager struct {
	trxContext store.TransactionContext
	store      api.DefinitionStore
}

func (d definitionManager) CreateOrchestrationDefinition(ctx context.Context, definition *api.OrchestrationDefinition) (*api.OrchestrationDefinition, error) {
	return store.Trx[api.OrchestrationDefinition](d.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.OrchestrationDefinition, error) {
		var missingErrors []error

		// Verify that all referenced activities exist
		for _, activity := range definition.Activities {
			exists, err := d.store.ExistsActivityDefinition(ctx, activity.Type)
			if err != nil {
				return nil, err
			}
			if !exists {
				missingErrors = append(missingErrors, types.NewClientError("activity type '%s' not found", activity.Type))
			}
		}

		if len(missingErrors) > 0 {
			return nil, errors.Join(missingErrors...)
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
