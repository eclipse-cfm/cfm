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

package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"iter"

	"github.com/metaform/connector-fabric-manager/common/collection"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/sqlstore"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
)

// PostgresDefinitionStore implements api.DefinitionStore for PostgreSQL
type PostgresDefinitionStore struct {
	orchestrationStore store.EntityStore[*api.OrchestrationDefinition]
	activityStore      store.EntityStore[*api.ActivityDefinition]
}

// NewPostgresDefinitionStore creates a new PostgresDefinitionStore instance
func newPostgresDefinitionStore() api.DefinitionStore {
	return &PostgresDefinitionStore{
		orchestrationStore: newOrchestrationStore(),
		activityStore:      newActivityStore(),
	}
}

func newOrchestrationStore() store.EntityStore[*api.OrchestrationDefinition] {
	columnNames := []string{"id", "type", "version", "description", "active", "schema", "activities", "templateref"}
	builder := sqlstore.NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
		"schema":     sqlstore.JSONBFieldTypeArrayOfObjects,
		"activities": sqlstore.JSONBFieldTypeArrayOfObjects,
	})

	estore := sqlstore.NewPostgresEntityStore[*api.OrchestrationDefinition](
		cfmOrchestrationDefinitionsTable,
		columnNames,
		recordToOrchestrationEntity,
		orchestrationEntityToRecord,
		builder,
	)

	return estore
}

func newActivityStore() store.EntityStore[*api.ActivityDefinition] {
	columnNames := []string{"id", "type", "version", "description", "input_schema", "output_schema"}
	builder := sqlstore.NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
		"inputSchema":  sqlstore.JSONBFieldTypeArrayOfObjects,
		"outputSchema": sqlstore.JSONBFieldTypeArrayOfObjects,
	}).WithFieldMappings(map[string]string{
		"inputSchema":  "input_schema",
		"outputSchema": "output_schema",
	})

	estore := sqlstore.NewPostgresEntityStore[*api.ActivityDefinition](
		cfmActivityDefinitionsTable,
		columnNames,
		recordToActivityEntity,
		activityEntityToRecord,
		builder,
	)

	return estore
}

// FindOrchestrationDefinition retrieves the OrchestrationDefinition associated with the given type
func (p *PostgresDefinitionStore) FindOrchestrationDefinition(
	ctx context.Context,
	orchestrationType model.OrchestrationType,
) (*api.OrchestrationDefinition, error) {
	return p.orchestrationStore.FindByID(ctx, string(orchestrationType))
}

// FindOrchestrationDefinitionsByPredicate retrieves OrchestrationDefinition instances matching the given predicate
func (p *PostgresDefinitionStore) FindOrchestrationDefinitionsByPredicate(
	ctx context.Context,
	predicate query.Predicate,
) iter.Seq2[api.OrchestrationDefinition, error] {
	return func(yield func(api.OrchestrationDefinition, error) bool) {
		for definition, err := range p.orchestrationStore.FindByPredicate(ctx, predicate) {
			if definition == nil {
				if !yield(api.OrchestrationDefinition{}, err) {
					return
				}
			} else {
				if !yield(*definition, err) {
					return
				}
			}
		}
	}
}

// ExistsOrchestrationDefinition returns true if an OrchestrationDefinition exists for the given type
func (p *PostgresDefinitionStore) ExistsOrchestrationDefinition(
	ctx context.Context,
	orchestrationType model.OrchestrationType,
) (bool, error) {
	return p.orchestrationStore.Exists(ctx, string(orchestrationType))
}

// FindActivityDefinition retrieves the ActivityDefinition associated with the given type
func (p *PostgresDefinitionStore) FindActivityDefinition(
	ctx context.Context,
	activityType api.ActivityType,
) (*api.ActivityDefinition, error) {
	return p.activityStore.FindByID(ctx, string(activityType))
}

// FindActivityDefinitionsByPredicate retrieves ActivityDefinition instances matching the given predicate
func (p *PostgresDefinitionStore) FindActivityDefinitionsByPredicate(
	ctx context.Context,
	predicate query.Predicate,
) iter.Seq2[api.ActivityDefinition, error] {
	return func(yield func(api.ActivityDefinition, error) bool) {
		for definition, err := range p.activityStore.FindByPredicate(ctx, predicate) {
			if definition == nil {
				if !yield(api.ActivityDefinition{}, err) {
					return
				}
			} else {
				if !yield(*definition, err) {
					return
				}
			}
		}
	}
}

// ExistsActivityDefinition returns true if an ActivityDefinition exists for the given type
func (p *PostgresDefinitionStore) ExistsActivityDefinition(
	ctx context.Context,
	activityType api.ActivityType,
) (bool, error) {
	return p.activityStore.Exists(ctx, string(activityType))
}

// StoreOrchestrationDefinition saves or updates an OrchestrationDefinition
func (p *PostgresDefinitionStore) StoreOrchestrationDefinition(
	ctx context.Context,
	definition *api.OrchestrationDefinition,
) (*api.OrchestrationDefinition, error) {
	// Check if it exists
	exists, err := p.orchestrationStore.Exists(ctx, definition.GetID())
	if err != nil {
		return nil, err
	}

	if exists {
		err = p.orchestrationStore.Update(ctx, definition)
		if err != nil {
			return nil, err
		}
		return p.orchestrationStore.FindByID(ctx, definition.GetID())
	}

	return p.orchestrationStore.Create(ctx, definition)
}

// StoreActivityDefinition saves or updates an ActivityDefinition
func (p *PostgresDefinitionStore) StoreActivityDefinition(
	ctx context.Context,
	definition *api.ActivityDefinition,
) (*api.ActivityDefinition, error) {
	// Check if it exists
	exists, err := p.activityStore.Exists(ctx, definition.GetID())
	if err != nil {
		return nil, err
	}

	if exists {
		err = p.activityStore.Update(ctx, definition)
		if err != nil {
			return nil, err
		}
		return p.activityStore.FindByID(ctx, definition.GetID())
	}

	return p.activityStore.Create(ctx, definition)
}

// DeleteOrchestrationDefinition removes an OrchestrationDefinition for the given type
func (p *PostgresDefinitionStore) DeleteOrchestrationDefinition(
	ctx context.Context,
	orchestrationType model.OrchestrationType,
) (bool, error) {
	err := p.orchestrationStore.Delete(ctx, string(orchestrationType))
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteActivityDefinition removes an ActivityDefinition for the given type
func (p *PostgresDefinitionStore) DeleteActivityDefinition(
	ctx context.Context,
	activityType api.ActivityType,
) (bool, error) {
	err := p.activityStore.Delete(ctx, string(activityType))
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ActivityDefinitionReferences returns references to an activity definition
func (p *PostgresDefinitionStore) ActivityDefinitionReferences(
	ctx context.Context,
	activityType api.ActivityType,
) ([]string, error) {

	var references []string

	for ref, err := range p.orchestrationStore.FindByPredicate(ctx, query.Eq("activities.type", activityType.String())) {
		if err != nil {
			return nil, err
		}

		if ref != nil {
			references = append(references, ref.Type.String())
		}
	}

	return references, nil
}

// ListOrchestrationDefinitions returns all OrchestrationDefinition instances
func (p *PostgresDefinitionStore) ListOrchestrationDefinitions(
	ctx context.Context,
) ([]api.OrchestrationDefinition, error) {
	return collection.CollectAllDeref(p.orchestrationStore.GetAll(ctx))
}

// ListActivityDefinitions returns all ActivityDefinition instances
func (p *PostgresDefinitionStore) ListActivityDefinitions(
	ctx context.Context,
) ([]api.ActivityDefinition, error) {
	return collection.CollectAllDeref(p.activityStore.GetAll(ctx))
}

func orchestrationEntityToRecord(definition *api.OrchestrationDefinition) (*sqlstore.DatabaseRecord, error) {
	record := &sqlstore.DatabaseRecord{
		Values: make(map[string]any),
	}

	record.Values["id"] = definition.Type
	record.Values["type"] = definition.Type
	record.Values["version"] = definition.Version
	record.Values["description"] = definition.Description
	record.Values["active"] = definition.Active
	record.Values["templateref"] = definition.TemplateRef

	if definition.Schema != nil {
		bytes, err := json.Marshal(definition.Schema)
		if err != nil {
			return record, err
		}
		record.Values["schema"] = bytes
	}

	if definition.Activities != nil {
		bytes, err := json.Marshal(definition.Activities)
		if err != nil {
			return record, err
		}
		record.Values["activities"] = bytes
	}

	return record, nil
}

func recordToOrchestrationEntity(tx *sql.Tx, record *sqlstore.DatabaseRecord) (*api.OrchestrationDefinition, error) {
	definition := &api.OrchestrationDefinition{}
	if otype, ok := record.Values["type"].(string); ok {
		definition.Type = model.OrchestrationType(otype)
	} else {
		return nil, fmt.Errorf("invalid orchestration definition type reading record")
	}

	if version, ok := record.Values["version"].(int64); ok {
		definition.Version = version
	} else {
		return nil, fmt.Errorf("invalid orchestration definition version reading record")
	}

	if description, ok := record.Values["description"].(string); ok {
		definition.Description = description
	} else {
		return nil, fmt.Errorf("invalid orchestration definition description reading record")
	}

	if active, ok := record.Values["active"].(bool); ok {
		definition.Active = active
	} else {
		return nil, fmt.Errorf("invalid orchestration definition active reading record")
	}

	if bytes, ok := record.Values["schema"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &definition.Schema); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["activities"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &definition.Activities); err != nil {
			return nil, err
		}
	}

	if templateref, ok := record.Values["templateref"].(string); ok {
		definition.TemplateRef = templateref
	} else {
		return nil, fmt.Errorf("invalid orchestration definition templateref reading record")
	}
	return definition, nil
}

func activityEntityToRecord(definition *api.ActivityDefinition) (*sqlstore.DatabaseRecord, error) {
	record := &sqlstore.DatabaseRecord{
		Values: make(map[string]any),
	}

	record.Values["id"] = definition.Type
	record.Values["type"] = definition.Type
	record.Values["version"] = definition.Version
	record.Values["description"] = definition.Description

	if definition.InputSchema != nil {
		bytes, err := json.Marshal(definition.InputSchema)
		if err != nil {
			return record, err
		}
		record.Values["input_schema"] = bytes
	}

	if definition.OutputSchema != nil {
		bytes, err := json.Marshal(definition.OutputSchema)
		if err != nil {
			return record, err
		}
		record.Values["output_schema"] = bytes
	}

	return record, nil
}

func recordToActivityEntity(tx *sql.Tx, record *sqlstore.DatabaseRecord) (*api.ActivityDefinition, error) {
	definition := &api.ActivityDefinition{}
	if otype, ok := record.Values["type"].(string); ok {
		definition.Type = api.ActivityType(otype)
	} else {
		return nil, fmt.Errorf("invalid activity definition type reading record")
	}

	if version, ok := record.Values["version"].(int64); ok {
		definition.Version = version
	} else {
		return nil, fmt.Errorf("invalid activity definition version reading record")
	}

	if description, ok := record.Values["description"].(string); ok {
		definition.Description = description
	} else {
		return nil, fmt.Errorf("invalid activity definition description reading record")
	}

	if bytes, ok := record.Values["input_schema"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &definition.InputSchema); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["output_schema"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &definition.OutputSchema); err != nil {
			return nil, err
		}
	}
	return definition, nil

}
