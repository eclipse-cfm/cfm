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
	"context"
	"iter"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/system"
)

const (
	OrchestrationIndexKey system.ServiceType = "pmstore:OrchestrationIndex"
)

// DefinitionStore manages OrchestrationDefinition and ActivityDefinitions.
//
// A definition is identified by its namespace and its type. Every method only reads or changes the definitions of the
// given namespace, or of the namespace of the given definition. An empty namespace is model.DefaultNamespace.
type DefinitionStore interface {

	// FindOrchestrationDefinition retrieves the OrchestrationDefinition associated with the given namespace and type.
	// Returns the OrchestrationDefinition object or store.ErrNotFound if the definition cannot be found.
	FindOrchestrationDefinition(ctx context.Context, namespace string, orchestrationType model.OrchestrationType) (*OrchestrationDefinition, error)

	// FindOrchestrationDefinitionsByPredicate retrieves the OrchestrationDefinition instances of the namespace that match
	// the given predicate.
	FindOrchestrationDefinitionsByPredicate(ctx context.Context, namespace string, predicate query.Predicate) iter.Seq2[OrchestrationDefinition, error]

	// ExistsOrchestrationDefinition returns true if an OrchestrationDefinition exists for the given namespace and type.
	ExistsOrchestrationDefinition(ctx context.Context, namespace string, orchestrationType model.OrchestrationType) (bool, error)

	// FindActivityDefinition retrieves the ActivityDefinition associated with the given namespace and type.
	// Returns the ActivityDefinition object or store.ErrNotFound if the definition cannot be found.
	FindActivityDefinition(ctx context.Context, namespace string, activityType ActivityType) (*ActivityDefinition, error)

	// FindActivityDefinitionsByPredicate retrieves the ActivityDefinition instances of the namespace that match the given
	// predicate.
	FindActivityDefinitionsByPredicate(ctx context.Context, namespace string, predicate query.Predicate) iter.Seq2[ActivityDefinition, error]

	// ExistsActivityDefinition returns true if an ActivityDefinition exists for the given namespace and type.
	ExistsActivityDefinition(ctx context.Context, namespace string, activityType ActivityType) (bool, error)

	// StoreOrchestrationDefinition saves or updates an OrchestrationDefinition in the namespace of the definition
	StoreOrchestrationDefinition(ctx context.Context, definition *OrchestrationDefinition) (*OrchestrationDefinition, error)

	// StoreActivityDefinition saves or updates an ActivityDefinition in the namespace of the definition
	StoreActivityDefinition(ctx context.Context, definition *ActivityDefinition) (*ActivityDefinition, error)

	// DeleteOrchestrationDefinition removes the OrchestrationDefinition for the given namespace and type, returning true
	// if successful.
	DeleteOrchestrationDefinition(ctx context.Context, namespace string, orchestrationType model.OrchestrationType) (bool, error)

	// ActivityDefinitionReferences returns the types of the orchestration definitions of the namespace that use the
	// given activity type.
	ActivityDefinitionReferences(ctx context.Context, namespace string, activityType ActivityType) ([]string, error)

	// DeleteActivityDefinition removes the ActivityDefinition for the given namespace and type, returning true if
	// successful.
	DeleteActivityDefinition(ctx context.Context, namespace string, activityType ActivityType) (bool, error)

	// ListOrchestrationDefinitions returns the OrchestrationDefinition instances of the namespace
	ListOrchestrationDefinitions(ctx context.Context, namespace string) ([]OrchestrationDefinition, error)

	// ListActivityDefinitions returns the ActivityDefinition instances of the namespace
	ListActivityDefinitions(ctx context.Context, namespace string) ([]ActivityDefinition, error)
}

type OrchestrationEntry struct {
	ID string `json:"id"`
	// Namespace is the namespace of the orchestration
	Namespace         string                  `json:"namespace"`
	Version           int64                   `json:"version"`
	CorrelationID     string                  `json:"correlationId"`
	State             OrchestrationState      `json:"state"`
	StateTimestamp    time.Time               `json:"stateTimestamp"`
	CreatedTimestamp  time.Time               `json:"createdTimestamp"`
	OrchestrationType model.OrchestrationType `json:"orchestrationType"`
	DefinitionID      string                  `json:"definitionId"`
}

func (o *OrchestrationEntry) GetID() string {
	return o.ID
}

func (o *OrchestrationEntry) GetVersion() int64 {
	return o.Version
}

func (o *OrchestrationEntry) IncrementVersion() {
	o.Version++
}
