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
	"iter"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
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

	if manifest.Namespace == "" {
		manifest.Namespace = model.DefaultNamespace
	} else if err := model.ValidateNamespace(manifest.Namespace); err != nil {
		return nil, types.NewClientError("%s", err.Error())
	}
	namespace := manifest.Namespace

	if manifest.ReplyTo != "" {
		if err := model.ValidateReplyTo(manifest.ReplyTo); err != nil {
			return nil, types.NewClientError("%s", err.Error())
		}
		if natsclient.IsReservedReplyTo(manifest.ReplyTo) {
			return nil, types.NewClientError("invalid replyTo %q: the subject is reserved", manifest.ReplyTo)
		}
	}

	var orchestration *api.Orchestration
	err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
		definition, err := p.store.FindOrchestrationDefinition(ctx, namespace, manifest.OrchestrationType)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				// Not found is a client error
				return types.NewClientError("orchestration type '%s' not found in namespace '%s'", manifest.OrchestrationType, namespace)
			}
			return types.NewFatalWrappedError(err, "unable to find orchestration definition for manifest %s", manifestID)
		}

		// perform de-duplication. The orchestrations of each namespace are stored under their own keys, so a manifest
		// only finds an orchestration of its own namespace.
		orch, err := p.orchestrator.GetOrchestration(ctx, namespace, manifestID)
		if err != nil {
			return types.NewFatalWrappedError(err, "error performing de-duplication for %s", manifestID)
		}

		if orch != nil {
			// The same manifest may be submitted again, for example when a start is retried. A different manifest that
			// reuses the ID is refused, so that it can neither read nor resume another caller's orchestration.
			if model.NamespaceOrDefault(orch.Namespace) != namespace || orch.OrchestrationType != manifest.OrchestrationType || orch.CorrelationID != manifest.CorrelationID {
				return fmt.Errorf("orchestration %s already exists with a different namespace, type or correlation ID: %w", manifestID, types.ErrConflict)
			}
			// Already exists, return its representation
			orchestration = orch
			if orch.State != api.OrchestrationStateInitialized {
				return nil
			}
			// Stored, but its start may not have completed, for example because the process died before its first
			// activities were enqueued. Executing it again starts it if that is the case.
			if err := p.orchestrator.Execute(ctx, orch); err != nil {
				return executionError(err, orch.ID, manifestID)
			}
			return nil
		}

		// Does not exist, create the orchestration
		orch, err = api.InstantiateOrchestration(manifest.ID, manifest.CorrelationID, manifest.OrchestrationType, definition.Type.String(), definition.Activities, manifest.Payload)
		if err != nil {
			return types.NewFatalWrappedError(err, "error instantiating orchestration for %s", manifestID)
		}
		orch.Namespace = namespace
		orch.CompensatedOrchestrationID = manifest.CompensatedManifestID
		orch.ReplyTo = manifest.ReplyTo
		err = p.orchestrator.Execute(ctx, orch)
		if err != nil {
			return executionError(err, orch.ID, manifestID)
		}
		orchestration = orch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return orchestration, nil
}

// executionError wraps an error from executing an orchestration. A recoverable error stays recoverable, so that the
// caller retries the start, which resumes the orchestration.
func executionError(err error, orchestrationID string, manifestID string) error {
	if types.IsRecoverable(err) {
		return types.NewRecoverableWrappedError(err, "error executing orchestration %s for %s", orchestrationID, manifestID)
	}
	return types.NewFatalWrappedError(err, "error executing orchestration %s for %s", orchestrationID, manifestID)
}

func (p provisionManager) Cancel(ctx context.Context, namespace string, orchestrationID string) error {
	//TODO implement me
	panic("implement me")
}

// GetOrchestration returns the orchestration if it belongs to the namespace. An orchestration of another namespace is
// reported as not found, so that its existence is not disclosed.
func (p provisionManager) GetOrchestration(ctx context.Context, namespace string, orchestrationID string) (*api.Orchestration, error) {
	orchestration, err := p.orchestrator.GetOrchestration(ctx, model.NamespaceOrDefault(namespace), orchestrationID)
	if err != nil || orchestration == nil {
		return orchestration, err
	}
	if model.NamespaceOrDefault(orchestration.Namespace) != model.NamespaceOrDefault(namespace) {
		return nil, nil
	}
	return orchestration, nil
}

func (p provisionManager) QueryOrchestrations(
	ctx context.Context,
	namespace string,
	predicate query.Predicate,
	options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
	predicate = namespacePredicate(namespace, predicate)
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

func (p provisionManager) CountOrchestrations(ctx context.Context, namespace string, predicate query.Predicate) (int64, error) {
	predicate = namespacePredicate(namespace, predicate)
	var count int64
	err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
		c, err := p.index.CountByPredicate(ctx, predicate)
		count = c
		return err
	})
	return count, err
}

// namespacePredicate restricts the predicate to the entities of the namespace. A nil predicate matches every entity of
// the namespace.
func namespacePredicate(namespace string, predicate query.Predicate) query.Predicate {
	namespaceEq := query.Eq("namespace", model.NamespaceOrDefault(namespace))
	if predicate == nil {
		return namespaceEq
	}
	return query.And(namespaceEq, predicate)
}
