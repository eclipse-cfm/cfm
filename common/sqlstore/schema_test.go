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
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createSchemaTestTable(db Executor) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_test (id TEXT PRIMARY KEY, data JSONB);
		CREATE INDEX IF NOT EXISTS idx_schema_test_data ON schema_test USING GIN (data)`)
	return err
}

func TestCreateSchema_Concurrent(t *testing.T) {
	defer CleanupTestData(t, testDB)

	const replicas = 8
	var wg sync.WaitGroup
	errs := make(chan error, replicas)
	for range replicas {
		wg.Go(func() { errs <- CreateSchema(testDB, createSchemaTestTable) })
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	var exists bool
	require.NoError(t, testDB.QueryRow("SELECT to_regclass('schema_test') IS NOT NULL").Scan(&exists))
	assert.True(t, exists)
}

func TestCreateSchema_RollsBackOnError(t *testing.T) {
	defer CleanupTestData(t, testDB)

	err := CreateSchema(testDB, func(db Executor) error {
		if err := createSchemaTestTable(db); err != nil {
			return err
		}
		return errors.New("forced error")
	})

	require.ErrorContains(t, err, "forced error")
	var exists bool
	require.NoError(t, testDB.QueryRow("SELECT to_regclass('schema_test') IS NOT NULL").Scan(&exists))
	assert.False(t, exists)
}
