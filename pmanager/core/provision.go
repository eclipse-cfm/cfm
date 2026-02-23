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
	"iter"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
)

type provisionManager struct {
	orchestrator api.Orchestrator
	store        api.DefinitionStore
	index        store.EntityStore[*api.OrchestrationEntry]
	trxContext   store.TransactionContext
	monitor      system.LogMonitor
}

func (p provisionManager) Start(ctx context.Context, manifest *model.OrchestrationManifest) (*api.Orchestration, error) {
	manifestID := manifest.ID

	// Validate required fields
	if manifest.ID == "" {
		return nil, types.NewClientError("Missing required field: id")
	}

	if manifest.OrchestrationType == "" {
		return nil, types.NewClientError("Missing required field: orchestrationType")
	}

	var orchestration *api.Orchestration
	err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
		definition, err := p.store.FindOrchestrationDefinition(ctx, manifest.OrchestrationType)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				// Not found is a client error
				return types.NewClientError("orchestration type '%s' not found", manifest.OrchestrationType)
			}
			return types.NewFatalWrappedError(err, "unable to find orchestration definition for manifest %s", manifestID)
		}

		// perform de-duplication
		orch, err := p.orchestrator.GetOrchestration(ctx, manifestID)
		if err != nil {
			return types.NewFatalWrappedError(err, "error performing de-duplication for %s", manifestID)
		}

		if orch != nil {
			// Already exists, return its representation
			orchestration = orch
			return nil
		}

		// Does not exist, create the orchestration
		orch, err = api.InstantiateOrchestration(manifest.ID, manifest.CorrelationID, manifest.OrchestrationType, definition.GetID(), definition.Activities, manifest.Payload)
		if err != nil {
			return types.NewFatalWrappedError(err, "error instantiating orchestration for %s", manifestID)
		}
		err = p.orchestrator.Execute(ctx, orch)
		if err != nil {
			return types.NewFatalWrappedError(err, "error executing orchestration %s for %s", orch.ID, manifestID)
		}
		orchestration = orch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return orchestration, nil
}

func (p provisionManager) Cancel(ctx context.Context, orchestrationID string) error {
	//TODO implement me
	panic("implement me")
}

func (p provisionManager) GetOrchestration(ctx context.Context, orchestrationID string) (*api.Orchestration, error) {
	return p.orchestrator.GetOrchestration(ctx, orchestrationID)
}

func (p provisionManager) QueryOrchestrations(
	ctx context.Context,
	predicate query.Predicate,
	options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
	return func(yield func(*api.OrchestrationEntry, error) bool) {
		err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
			for entry, err := range p.index.FindByPredicatePaginated(ctx, predicate, options) {
				if !yield(entry, err) {
					return context.Canceled
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			yield(nil, err)
		}
	}
}

func (p provisionManager) CountOrchestrations(ctx context.Context, predicate query.Predicate) (int64, error) {
	var count int64
	err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
		c, err := p.index.CountByPredicate(ctx, predicate)
		count = c
		return err
	})
	return count, err
}
