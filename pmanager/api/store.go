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

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/system"
)

const (
	OrchestrationIndexKey system.ServiceType = "pmstore:OrchestrationIndex"
)

// DefinitionStore manages OrchestrationDefinition and ActivityDefinitions.
type DefinitionStore interface {

	// FindOrchestrationDefinition retrieves the OrchestrationDefinition associated with the given type.
	// Returns the OrchestrationDefinition object or store.ErrNotFound if the definition cannot be found.
	FindOrchestrationDefinition(ctx context.Context, orchestrationType model.OrchestrationType) (*OrchestrationDefinition, error)

	// FindOrchestrationDefinitionsByPredicate retrieves OrchestrationDefinition instances matching the given predicate.
	FindOrchestrationDefinitionsByPredicate(ctx context.Context, predicate query.Predicate) iter.Seq2[OrchestrationDefinition, error]

	// ExistsOrchestrationDefinition returns true if an OrchestrationDefinition exists for the given type.
	ExistsOrchestrationDefinition(ctx context.Context, orchestrationType model.OrchestrationType) (bool, error)

	// FindActivityDefinition retrieves the ActivityDefinition associated with the given type.
	// Returns the ActivityDefinition object or store.ErrNotFound if the definition cannot be found.
	FindActivityDefinition(ctx context.Context, activityType ActivityType) (*ActivityDefinition, error)

	// FindActivityDefinitionsByPredicate retrieves ActivityDefinition instances matching the given predicate.
	FindActivityDefinitionsByPredicate(ctx context.Context, predicate query.Predicate) iter.Seq2[ActivityDefinition, error]

	// ExistsActivityDefinition returns true if an ActivityDefinition exists for the given type.
	ExistsActivityDefinition(ctx context.Context, activityType ActivityType) (bool, error)

	// StoreOrchestrationDefinition saves or updates an OrchestrationDefinition
	StoreOrchestrationDefinition(ctx context.Context, definition *OrchestrationDefinition) (*OrchestrationDefinition, error)

	// StoreActivityDefinition saves or updates an ActivityDefinition
	StoreActivityDefinition(ctx context.Context, definition *ActivityDefinition) (*ActivityDefinition, error)

	// DeleteOrchestrationDefinition removes an OrchestrationDefinition for the given type, returning true if successful.
	DeleteOrchestrationDefinition(ctx context.Context, orchestrationType model.OrchestrationType) (bool, error)

	ActivityDefinitionReferences(ctx context.Context, activityType ActivityType) ([]string, error)

	// DeleteActivityDefinition removes an ActivityDefinition for the given type, returning true if successful.
	DeleteActivityDefinition(ctx context.Context, activityType ActivityType) (bool, error)

	// ListOrchestrationDefinitions returns OrchestrationDefinition instances
	ListOrchestrationDefinitions(ctx context.Context) ([]OrchestrationDefinition, error)

	// ListActivityDefinitions returns ActivityDefinition instances
	ListActivityDefinitions(ctx context.Context) ([]ActivityDefinition, error)
}

type OrchestrationEntry struct {
	ID                string                  `json:"id"`
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
