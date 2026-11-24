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

package memorystore

import (
	"context"
	"iter"
	"sync"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
)

var defaultMatcher = &query.DefaultFieldMatcher{}

// MemoryDefinitionStore is a thread-safe in-memory store for orchestration and activity definitions.
type MemoryDefinitionStore struct {
	mutex                    sync.RWMutex
	orchestrationDefinitions map[string]*api.OrchestrationDefinition
	activityDefinitions      map[string]*api.ActivityDefinition
}

// NewDefinitionStore creates a new thread-safe in-memory definition store
func NewDefinitionStore() *MemoryDefinitionStore {
	return &MemoryDefinitionStore{
		orchestrationDefinitions: make(map[string]*api.OrchestrationDefinition),
		activityDefinitions:      make(map[string]*api.ActivityDefinition),
	}
}

func (d *MemoryDefinitionStore) FindOrchestrationDefinition(
	_ context.Context,
	namespace string,
	orchestrationType model.OrchestrationType) (*api.OrchestrationDefinition, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	definition, exists := d.orchestrationDefinitions[api.DefinitionKey(namespace, orchestrationType.String())]
	if !exists {
		return nil, types.ErrNotFound
	}

	// Return a copy to prevent external modifications
	definitionCopy := *definition
	return &definitionCopy, nil
}

func (d *MemoryDefinitionStore) FindOrchestrationDefinitionsByPredicate(
	_ context.Context,
	namespace string,
	predicate query.Predicate) iter.Seq2[api.OrchestrationDefinition, error] {
	return findDefinitionsByPredicate(d, namespacePredicate(namespace, predicate), d.orchestrationDefinitions)
}

func (d *MemoryDefinitionStore) ExistsOrchestrationDefinition(
	_ context.Context,
	namespace string,
	orchestrationType model.OrchestrationType) (bool, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	_, exists := d.orchestrationDefinitions[api.DefinitionKey(namespace, orchestrationType.String())]
	return exists, nil
}

func (d *MemoryDefinitionStore) FindActivityDefinition(_ context.Context, namespace string, activityType api.ActivityType) (*api.ActivityDefinition, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	definition, exists := d.activityDefinitions[api.DefinitionKey(namespace, activityType.String())]
	if !exists {
		return nil, types.ErrNotFound
	}

	// Return a copy to prevent external modifications
	definitionCopy := *definition
	return &definitionCopy, nil
}

func (d *MemoryDefinitionStore) FindActivityDefinitionsByPredicate(
	_ context.Context,
	namespace string,
	predicate query.Predicate) iter.Seq2[api.ActivityDefinition, error] {
	return findDefinitionsByPredicate(d, namespacePredicate(namespace, predicate), d.activityDefinitions)
}

func (d *MemoryDefinitionStore) ExistsActivityDefinition(_ context.Context, namespace string, activityType api.ActivityType) (bool, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	_, exists := d.activityDefinitions[api.DefinitionKey(namespace, activityType.String())]
	return exists, nil
}

func (d *MemoryDefinitionStore) StoreOrchestrationDefinition(
	_ context.Context,
	definition *api.OrchestrationDefinition) (*api.OrchestrationDefinition, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.orchestrationDefinitions[definition.GetID()] != nil {
		return nil, types.ErrConflict
	}

	// Store a copy to prevent external modifications
	definitionCopy := *definition
	definitionCopy.Namespace = model.NamespaceOrDefault(definition.Namespace)
	d.orchestrationDefinitions[definitionCopy.GetID()] = &definitionCopy
	result := definitionCopy
	return &result, nil
}

func (d *MemoryDefinitionStore) StoreActivityDefinition(
	_ context.Context,
	definition *api.ActivityDefinition) (*api.ActivityDefinition, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.activityDefinitions[definition.GetID()] != nil {
		return nil, types.ErrConflict
	}
	// Store a copy to prevent external modifications
	definitionCopy := *definition
	definitionCopy.Namespace = model.NamespaceOrDefault(definition.Namespace)
	d.activityDefinitions[definitionCopy.GetID()] = &definitionCopy
	result := definitionCopy
	return &result, nil
}

func (d *MemoryDefinitionStore) DeleteOrchestrationDefinition(
	_ context.Context,
	namespace string,
	orchestrationType model.OrchestrationType) (bool, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	key := api.DefinitionKey(namespace, orchestrationType.String())
	_, exists := d.orchestrationDefinitions[key]
	if exists {
		delete(d.orchestrationDefinitions, key)
	}
	return exists, nil
}

func (d *MemoryDefinitionStore) ActivityDefinitionReferences(_ context.Context, namespace string, activityType api.ActivityType) ([]string, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	namespace = model.NamespaceOrDefault(namespace)
	results := make([]string, 0, len(d.orchestrationDefinitions))
	for _, oDefinition := range d.orchestrationDefinitions {
		if oDefinition.Namespace != namespace {
			continue
		}
		for _, aDefinition := range oDefinition.Activities {
			if aDefinition.Type == activityType {
				results = append(results, oDefinition.Type.String())
				break
			}
		}
	}
	return results, nil
}

func (d *MemoryDefinitionStore) DeleteActivityDefinition(_ context.Context, namespace string, activityType api.ActivityType) (bool, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	key := api.DefinitionKey(namespace, activityType.String())
	_, exists := d.activityDefinitions[key]
	if exists {
		delete(d.activityDefinitions, key)
	}
	return exists, nil
}

func (d *MemoryDefinitionStore) ListOrchestrationDefinitions(_ context.Context, namespace string) ([]api.OrchestrationDefinition, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	namespace = model.NamespaceOrDefault(namespace)
	return listDefinitions(d.orchestrationDefinitions, func(definition *api.OrchestrationDefinition) bool {
		return definition.Namespace == namespace
	})
}

func (d *MemoryDefinitionStore) ListActivityDefinitions(_ context.Context, namespace string) ([]api.ActivityDefinition, error) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	namespace = model.NamespaceOrDefault(namespace)
	return listDefinitions(d.activityDefinitions, func(definition *api.ActivityDefinition) bool {
		return definition.Namespace == namespace
	})
}

// namespacePredicate restricts the predicate to the definitions of the namespace
func namespacePredicate(namespace string, predicate query.Predicate) query.Predicate {
	namespaceEq := query.Eq("namespace", model.NamespaceOrDefault(namespace))
	if predicate == nil {
		return namespaceEq
	}
	return query.And(namespaceEq, predicate)
}

// Clear removes all stored definitions
func (d *MemoryDefinitionStore) Clear() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.orchestrationDefinitions = make(map[string]*api.OrchestrationDefinition)
	d.activityDefinitions = make(map[string]*api.ActivityDefinition)
}

// listDefinitions lists the definitions that the filter accepts
func listDefinitions[T any](definitionMap map[string]*T, filter func(*T) bool) ([]T, error) {
	definitions := make([]T, 0, len(definitionMap))
	for _, definition := range definitionMap {
		if !filter(definition) {
			continue
		}
		// Return a copy to prevent external modifications
		definitions = append(definitions, *definition)
	}
	return definitions, nil
}

// findDefinitionsByPredicate is a generic helper that filters definitions by predicate
func findDefinitionsByPredicate[T any](d *MemoryDefinitionStore, predicate query.Predicate, definitionMap map[string]*T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		d.mutex.RLock()
		defer d.mutex.RUnlock()

		for _, definition := range definitionMap {
			if predicate.Matches(definition, defaultMatcher) {
				// Return a copy to prevent external modifications
				definitionCopy := *definition
				if !yield(definitionCopy, nil) {
					return
				}
			}
		}
	}
}
