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

	"github.com/eclipse-cfm/cfm/common/sqlstore"
)

const (
	cfmTenantsTable             = "tenants"
	cfmCellsTable               = "cells"
	cfmParticipantProfilesTable = "participant_profiles"
	cfmDataspaceProfilesTable   = "dataspace_profiles"
)

// Note fields are quoted to avoid some IDEs (Goland) reformatting them to uppercase

func createTenantsTable(db sqlstore.Executor) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			version INT DEFAULT 1,
			properties JSONB
		);
		ALTER TABLE %s ADD COLUMN IF NOT EXISTS owner VARCHAR(255);
		CREATE INDEX IF NOT EXISTS idx_tenant_owner ON %s(owner)
	`, cfmTenantsTable, cfmTenantsTable, cfmTenantsTable))
	return err
}

func createParticipantProfilesTable(db sqlstore.Executor) error {
	_, err := db.Exec(fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(255) PRIMARY KEY,
				version BIGINT NOT NULL,
				identifier VARCHAR(255),
				tenant_id VARCHAR(255),
			    participant_roles JSONB,
				dataspace_profile_ids JSONB,
				vpas JSONB,
				error BOOLEAN,
				error_detail TEXT,
				properties JSONB
			);
			CREATE INDEX IF NOT EXISTS idx_participant_tenant ON %s(tenant_id);
			ALTER TABLE %s ADD COLUMN IF NOT EXISTS owner VARCHAR(255);
			CREATE INDEX IF NOT EXISTS idx_participant_owner ON %s(owner);
			ALTER TABLE %s ADD COLUMN IF NOT EXISTS error_cause TEXT
	`, cfmParticipantProfilesTable, cfmParticipantProfilesTable, cfmParticipantProfilesTable, cfmParticipantProfilesTable, cfmParticipantProfilesTable))
	return err
}

func createDataspaceProfilesTable(db sqlstore.Executor) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			version INT DEFAULT 1,
			dataspace_spec JSONB,
			artifacts JSONB,
			deployments JSONB,
			properties JSONB
		)
	`, cfmDataspaceProfilesTable))
	return err
}

func createCellsTable(db sqlstore.Executor) error {
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			external_id TEXT UNIQUE,
			version INT DEFAULT 1,
			"state" TEXT NOT NULL,
			state_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			properties JSONB
		)
	`, cfmCellsTable))
	return err
}
