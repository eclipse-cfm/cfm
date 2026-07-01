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
	"testing"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInitContext(dsn string) *system.InitContext {
	config := viper.New()
	if dsn != "" {
		config.Set(dsnKey, dsn)
	}
	return &system.InitContext{
		StartContext: system.StartContext{
			Registry:   system.NewServiceRegistry(),
			LogMonitor: system.NoopMonitor{},
			Config:     config,
			Mode:       system.DevelopmentMode,
		},
	}
}

// TestPostgresServiceAssembly_Init_Succeeds verifies that Init creates the tables and returns
// no error against a reachable database.
func TestPostgresServiceAssembly_Init_Succeeds(t *testing.T) {
	assembly := &PostgresServiceAssembly{}
	err := assembly.Init(newInitContext(testDSN))
	require.NoError(t, err)
	require.NoError(t, assembly.Finalize())
}

// TestPostgresServiceAssembly_Init_FailsWhenTablesCannotBeCreated verifies the fail-fast behavior:
// when the database is unreachable, table creation fails and Init returns the error instead of
// swallowing it and leaving the store permanently broken.
func TestPostgresServiceAssembly_Init_FailsWhenTablesCannotBeCreated(t *testing.T) {
	// Well-formed DSN pointing at an unreachable port. otelsql.Open connects lazily, so it
	// succeeds; the failure surfaces when createTables issues the first query.
	unreachableDSN := "postgres://user:pass@127.0.0.1:1/nonexistent?sslmode=disable&connect_timeout=1"

	assembly := &PostgresServiceAssembly{}
	err := assembly.Init(newInitContext(unreachableDSN))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create tables")
	require.NoError(t, assembly.Finalize())
}

// TestPostgresServiceAssembly_Init_FailsWithoutDSN verifies Init errors when the DSN is not configured.
func TestPostgresServiceAssembly_Init_FailsWithoutDSN(t *testing.T) {
	assembly := &PostgresServiceAssembly{}
	err := assembly.Init(newInitContext(""))

	require.Error(t, err)
	assert.Contains(t, err.Error(), dsnKey)
}
