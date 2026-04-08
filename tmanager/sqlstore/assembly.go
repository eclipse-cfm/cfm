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
	"github.com/eclipse-cfm/cfm/tmanager/api"

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
	return "Tenant Manager Postgres"
}

func (a *PostgresServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{api.CellStoreKey, api.DataspaceProfileStoreKey, api.ParticipantProfileStoreKey, store.TransactionContextKey}
}

func (a *PostgresServiceAssembly) Init(ictx *system.InitContext) error {
	if !ictx.Config.IsSet(dsnKey) {
		return fmt.Errorf("missing Postgres DSN configuration: %s", dsnKey)
	}
	dsn := ictx.Config.GetString(dsnKey)

	db, err := otelsql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("error connecting to DB at %s: %w", dsn, err)
	}

	a.db = db

	createTables(db)

	cellStore := newCellStore()
	dataspaceStore := newDataspaceProfileStore()
	participantStore := newParticipantProfileStore()
	tenantStore := newTenantStore()

	ictx.Registry.Register(api.TenantStoreKey, tenantStore)
	ictx.Registry.Register(api.ParticipantProfileStoreKey, participantStore)
	ictx.Registry.Register(api.DataspaceProfileStoreKey, dataspaceStore)
	ictx.Registry.Register(api.CellStoreKey, cellStore)

	txContext := sqlstore.NewDBTransactionContext(db)
	ictx.Registry.Register(store.TransactionContextKey, txContext)

	return nil
}

func (a *PostgresServiceAssembly) Finalize() error {
	if a.db != nil {
		a.db.Close()
	}
	return nil
}

func createTables(db *sql.DB) error {
	err := createCellsTable(db)
	if err != nil {
		return err
	}

	err = createDataspaceProfilesTable(db)
	if err != nil {
		return err
	}

	err = createParticipantProfilesTable(db)
	if err != nil {
		return err
	}

	err = createTenantsTable(db)

	if err != nil {
		return err
	}

	return nil
}
