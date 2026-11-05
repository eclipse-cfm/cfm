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

// schemaLockKey is the Postgres advisory lock that serializes schema creation. All CFM components use the same key, so
// that components sharing a database don't create their tables concurrently either.
const schemaLockKey int64 = 0x43464d5f534348 // "CFM_SCH"

// Executor runs a statement. Both *sql.DB and *sql.Tx implement it.
type Executor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// CreateSchema runs create in one transaction that first takes a Postgres advisory lock. Replicas that start at the same
// time then create their tables one after the other: concurrent CREATE TABLE IF NOT EXISTS statements for the same
// table can otherwise fail with a unique violation on the system catalog. create must be idempotent, as it runs on
// every start.
func CreateSchema(db *sql.DB, create func(Executor) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin schema transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("SELECT pg_advisory_xact_lock($1)", schemaLockKey); err != nil {
		return fmt.Errorf("failed to acquire schema lock: %w", err)
	}
	if err := create(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit schema: %w", err)
	}
	return nil
}
