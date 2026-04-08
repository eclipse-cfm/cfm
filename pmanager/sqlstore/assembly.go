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

	"github.com/XSAM/otelsql"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/telemetry"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	_ "github.com/lib/pq" // Register PostgreSQL driver
)

const (
	driverName = "postgres"
	dsnKey     = "dsn"
)

type PostgresServiceAssembly struct {
	system.DefaultServiceAssembly
	db *sql.DB
}

func (a *PostgresServiceAssembly) Name() string {
	return "Provision Manager Postgres"
}

func (a *PostgresServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{api.DefinitionStoreKey, api.OrchestrationIndexKey, store.TransactionContextKey}
}

func (a *PostgresServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{telemetry.TracerProviderKey}
}

func (a *PostgresServiceAssembly) Init(context *system.InitContext) error {
	context.Registry.Register(api.DefinitionStoreKey, newPostgresDefinitionStore())
	context.Registry.Register(api.OrchestrationIndexKey, newOrchestrationEntryStore())

	if !context.Config.IsSet(dsnKey) {
		return fmt.Errorf("missing Postgres DSN configuration: %s", dsnKey)
	}
	dsn := context.Config.GetString(dsnKey)

	db, err := otelsql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("error connecting to DB at %s: %w", dsn, err)
	}
	a.db = db

	txContext := sqlstore.NewDBTransactionContext(db)
	context.Registry.Register(store.TransactionContextKey, txContext)

	return createTables(db)
}

func (a *PostgresServiceAssembly) Finalize() error {
	if a.db != nil {
		a.db.Close()
	}
	return nil
}

func createTables(db *sql.DB) error {
	err := createActivityDefinitionsTable(db)
	if err != nil {
		return err
	}

	err = createOrchestrationDefinitionsTable(db)

	if err != nil {
		return err
	}

	err = createOrchestrationEntriesTable(db)

	if err != nil {
		return err
	}

	return nil
}
