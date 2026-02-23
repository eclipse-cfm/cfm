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
)

const (
	cfmOrchestrationEntriesTable     = "orchestration_entries"
	cfmOrchestrationDefinitionsTable = "orchestration_definitions"
	cfmActivityDefinitionsTable      = "activity_definitions"
)

// Note fields are quoted to avoid some IDEs (Goland) reformatting them to uppercase

func createOrchestrationEntriesTable(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			version BIGINT NOT NULL,
			correlation_id VARCHAR(255) NOT NULL ,
		    definition_id VARCHAR(255) NOT NULL,
			"state" INTEGER,
			state_timestamp TIMESTAMP NOT NULL ,
			created_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			orchestration_type VARCHAR(255)
		)
	`, cfmOrchestrationEntriesTable))
	return err
}

func createOrchestrationDefinitionsTable(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
		    id VARCHAR(255) PRIMARY KEY,
			"type" VARCHAR(255),
			version BIGINT NOT NULL,
			description TEXT,
			active BOOLEAN DEFAULT FALSE,
			"schema" JSONB,
			activities JSONB,
		    templateref TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_orchestration_type ON orchestration_definitions(TYPE)
	`, cfmOrchestrationDefinitionsTable))
	return err
}

func createActivityDefinitionsTable(db *sql.DB) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
		    id VARCHAR(255) PRIMARY KEY,
			"type" VARCHAR(255),
			version BIGINT NOT NULL,
			description TEXT,
			input_schema JSONB,
			output_schema JSONB
		)
	`, cfmActivityDefinitionsTable))
	return err
}
