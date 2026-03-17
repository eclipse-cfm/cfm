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
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const pgDb = "cfm"

// SetupTestContainer creates a PostgreSQL test container
// If t is nil, it's being called from TestMain and should panic on errors
func SetupTestContainer(t *testing.T) (testcontainers.Container, string, error) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       pgDb,
		},
		WaitingFor: wait.ForAll(wait.ForExposedPort(), wait.ForListeningPort("5432/tcp")),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		if t != nil {
			t.Fatalf("Failed to start container: %v", err)
		}
		panic(err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		if t != nil {
			t.Fatalf("Failed to get container host: %v", err)
		}
		panic(err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		if t != nil {
			t.Fatalf("Failed to get container port: %v", err)
		}
		panic(err)
	}

	dsn := fmt.Sprintf("postgres://test:test@%s:%s/%s?sslmode=disable", host, port.Port(), pgDb)
	return container, dsn, nil
}

// CleanupTestData removes all data from test tables
func CleanupTestData(t *testing.T, db *sql.DB) {
	if db == nil {
		return
	}
	_, err := db.Exec(`
		DO $$ DECLARE
			r RECORD;
		BEGIN
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
				EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
			END LOOP;
		END $$;
	`)
	if err != nil {
		// Don't fail the test, just log the warning
		t.Logf("Warning: Failed to drop tables: %v", err)
	}
}
