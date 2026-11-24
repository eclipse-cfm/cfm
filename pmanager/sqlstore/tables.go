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
	"fmt"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
)

const (
	cfmOrchestrationEntriesTable     = "orchestration_entries"
	cfmOrchestrationDefinitionsTable = "orchestration_definitions"
	cfmActivityDefinitionsTable      = "activity_definitions"
)

// Note fields are quoted to avoid some IDEs (Goland) reformatting them to uppercase

func createOrchestrationEntriesTable(db sqlstore.Executor) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %[1]s (
			id VARCHAR(255) PRIMARY KEY,
			namespace VARCHAR(255) NOT NULL DEFAULT '%[2]s',
			version BIGINT NOT NULL,
			correlation_id VARCHAR(255) NOT NULL ,
		    definition_id VARCHAR(255) NOT NULL,
			"state" INTEGER,
			state_timestamp TIMESTAMP NOT NULL ,
			created_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			orchestration_type VARCHAR(255)
		);
		-- entries created before namespaces were introduced belong to the default namespace
		ALTER TABLE %[1]s ADD COLUMN IF NOT EXISTS namespace VARCHAR(255) NOT NULL DEFAULT '%[2]s';
		CREATE INDEX IF NOT EXISTS idx_orchestration_entries_namespace ON %[1]s(namespace)
	`, cfmOrchestrationEntriesTable, model.DefaultNamespace))
	return err
}

func createOrchestrationDefinitionsTable(db sqlstore.Executor) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %[1]s (
		    id VARCHAR(255) PRIMARY KEY,
			namespace VARCHAR(255) NOT NULL DEFAULT '%[2]s',
			"type" VARCHAR(255),
			version BIGINT NOT NULL,
			description TEXT,
			active BOOLEAN DEFAULT FALSE,
			"schema" JSONB,
			activities JSONB,
		    templateref TEXT,
			compensation_type VARCHAR(255)
		);
		CREATE INDEX IF NOT EXISTS idx_orchestration_type ON %[1]s(TYPE)
	`, cfmOrchestrationDefinitionsTable, model.DefaultNamespace))
	if err != nil {
		return err
	}
	return migrateDefinitionKeys(db, cfmOrchestrationDefinitionsTable)
}

func createActivityDefinitionsTable(db sqlstore.Executor) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %[1]s (
		    id VARCHAR(255) PRIMARY KEY,
			namespace VARCHAR(255) NOT NULL DEFAULT '%[2]s',
			"type" VARCHAR(255),
			version BIGINT NOT NULL,
			description TEXT,
			input_schema JSONB,
			output_schema JSONB
		)
	`, cfmActivityDefinitionsTable, model.DefaultNamespace))
	if err != nil {
		return err
	}
	return migrateDefinitionKeys(db, cfmActivityDefinitionsTable)
}

// migrateDefinitionKeys moves the definitions that were stored before namespaces were introduced into the default
// namespace. Their key was the type alone, and is changed to the key of the type in the default namespace, see
// api.DefinitionKey. A type cannot contain the key separator, so a key without it was not migrated yet, and the
// migration can run on every start.
func migrateDefinitionKeys(db sqlstore.Executor, table string) error {
	_, err := db.Exec(fmt.Sprintf(`
		ALTER TABLE %[1]s ADD COLUMN IF NOT EXISTS namespace VARCHAR(255) NOT NULL DEFAULT '%[2]s';
		UPDATE %[1]s SET id = namespace || '/' || "type" WHERE POSITION('/' IN id) = 0;
		CREATE INDEX IF NOT EXISTS idx_%[1]s_namespace_type ON %[1]s(namespace, "type")
	`, table, model.DefaultNamespace))
	return err
}
