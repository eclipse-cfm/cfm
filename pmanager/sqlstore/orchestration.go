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
	"database/sql"
	"fmt"
	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/sqlstore"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
)

func newOrchestrationEntryStore() store.EntityStore[*api.OrchestrationEntry] {
	columnNames := []string{"id", "version", "correlation_id", "definition_id", "state", "state_timestamp", "created_timestamp", "orchestration_type"}
	builder := sqlstore.NewPostgresJSONBBuilder().
		WithFieldMappings(map[string]string{"correlationId": "correlation_id",
			"DefinitionID":      "definition_id",
			"stateTimestamp":    "state_timestamp",
			"createdTimestamp":  "created_timestamp",
			"orchestrationType": "orchestration_type"})

	estore := sqlstore.NewPostgresEntityStore[*api.OrchestrationEntry](
		cfmOrchestrationEntriesTable,
		columnNames,
		recordToOrchestrationEntry,
		orchestrationEntryToRecord,
		builder,
	)

	return estore
}

func recordToOrchestrationEntry(tx *sql.Tx, record *sqlstore.DatabaseRecord) (*api.OrchestrationEntry, error) {
	entry := &api.OrchestrationEntry{}
	if id, ok := record.Values["id"].(string); ok {
		entry.ID = id
	} else {
		return nil, fmt.Errorf("invalid orchestration entry id reading record")
	}

	if version, ok := record.Values["version"].(int64); ok {
		entry.Version = version
	} else {
		return nil, fmt.Errorf("invalid orchestration entry version reading record")
	}

	if correlationId, ok := record.Values["correlation_id"].(string); ok {
		entry.CorrelationID = correlationId
	} else {
		return nil, fmt.Errorf("invalid orchestration entry correlation_id reading record")
	}

	if definitionId, ok := record.Values["definition_id"].(string); ok {
		entry.DefinitionID = definitionId
	} else {
		return nil, fmt.Errorf("invalid orchestration entry definition_id reading record")
	}

	if state, ok := record.Values["state"].(int64); ok {
		entry.State = api.OrchestrationState(state)
	} else {
		return nil, fmt.Errorf("invalid orchestration entry state reading record")
	}

	if timestamp, ok := record.Values["state_timestamp"].(time.Time); ok {
		entry.StateTimestamp = timestamp
	} else {
		return nil, fmt.Errorf("invalid orchestration entry state_timestamp reading record")
	}

	if timestamp, ok := record.Values["created_timestamp"].(time.Time); ok {
		entry.CreatedTimestamp = timestamp
	} else {
		return nil, fmt.Errorf("invalid orchestration entry created_timestamp reading record")
	}

	if otype, ok := record.Values["orchestration_type"].(string); ok {
		entry.OrchestrationType = model.OrchestrationType(otype)
	} else {
		return nil, fmt.Errorf("invalid orchestration entry type reading record")
	}

	return entry, nil

}

func orchestrationEntryToRecord(orchestrationEntry *api.OrchestrationEntry) (*sqlstore.DatabaseRecord, error) {
	record := &sqlstore.DatabaseRecord{
		Values: make(map[string]any),
	}

	record.Values["id"] = orchestrationEntry.ID
	record.Values["version"] = orchestrationEntry.Version
	record.Values["correlation_id"] = orchestrationEntry.CorrelationID
	record.Values["state"] = orchestrationEntry.State
	record.Values["state_timestamp"] = orchestrationEntry.StateTimestamp
	record.Values["created_timestamp"] = orchestrationEntry.CreatedTimestamp
	record.Values["orchestration_type"] = orchestrationEntry.OrchestrationType
	record.Values["definition_id"] = orchestrationEntry.DefinitionID
	return record, nil
}
