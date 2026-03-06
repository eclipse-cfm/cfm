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
	"os"
	"testing"

	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/testcontainers/testcontainers-go"
)

var (
	testContainer testcontainers.Container
	testDB        *sql.DB
	testDSN       string
)

// TestMain runs before all tests in the package
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start PostgreSQL container once
	var err error
	testContainer, testDSN, err = sqlstore.SetupTestContainer(nil)
	if err != nil {
		panic(err)
	}

	// Create DB connection
	testDB, err = sql.Open("postgres", testDSN)
	if err != nil {
		panic(err)
	}
	defer testDB.Close()

	// Run all tests
	code := m.Run()

	// Cleanup
	testContainer.Terminate(ctx)
	os.Exit(code)
}
