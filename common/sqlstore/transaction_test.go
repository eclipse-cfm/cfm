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
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDBTable(t *testing.T) {
	_, err := testDB.Exec(`
		DROP TABLE IF EXISTS test_table CASCADE;
		CREATE TABLE test_table (
			id SERIAL PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	require.NoError(t, err)
}

func TestDBTransactionContext(t *testing.T) {
	setupTestDBTable(t)
	defer CleanupTestData(t, testDB)

	ctx := context.Background()
	trxContext := NewDBTransactionContext(testDB)

	t.Run("Successful transaction", func(t *testing.T) {
		err := trxContext.Execute(ctx, func(ctx context.Context) error {
			trx := ctx.Value(SQLTransactionKey).(*sql.Tx)
			_, err := trx.Exec("INSERT INTO test_table (value) VALUES ($1)", "test1")
			return err
		})

		assert.NoError(t, err)

		// Verify the data was inserted
		var count int
		err = testDB.QueryRow("SELECT COUNT(*) FROM test_table WHERE value = $1", "test1").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Failed transaction should rollback", func(t *testing.T) {
		initialCount := 0
		err := testDB.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&initialCount)
		assert.NoError(t, err)

		err = trxContext.Execute(ctx, func(ctx context.Context) error {
			trx := ctx.Value(SQLTransactionKey).(*sql.Tx)

			_, err := trx.Exec("INSERT INTO test_table (value) VALUES ($1)", "test2")
			if err != nil {
				return err
			}
			return errors.New("forced error")
		})

		assert.Error(t, err)

		// Verify the data was rolled back
		var count int
		err = testDB.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, initialCount, count)
	})

	t.Run("Panic should rollback", func(t *testing.T) {
		initialCount := 0
		err := testDB.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&initialCount)
		assert.NoError(t, err)

		assert.Panics(t, func() {
			_ = trxContext.Execute(ctx, func(ctx context.Context) error {
				trx := ctx.Value(SQLTransactionKey).(*sql.Tx)
				_, err := trx.Exec("INSERT INTO test_table (value) VALUES ($1)", "test3")
				if err != nil {
					return err
				}
				panic("forced panic")
			})
		})

		// Verify the data was rolled back
		var count int
		err = testDB.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, initialCount, count)
	})
}

func TestDBTransactionContext_NestedExecuteJoinsTransaction(t *testing.T) {
	setupTestDBTable(t)
	defer CleanupTestData(t, testDB)
	trxContext := NewDBTransactionContext(testDB)

	count := func() int {
		var count int
		require.NoError(t, testDB.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count))
		return count
	}

	t.Run("inner call runs in the outer transaction", func(t *testing.T) {
		err := trxContext.Execute(t.Context(), func(outer context.Context) error {
			outerTx := outer.Value(SQLTransactionKey).(*sql.Tx)
			return trxContext.Execute(outer, func(inner context.Context) error {
				assert.Same(t, outerTx, inner.Value(SQLTransactionKey).(*sql.Tx))
				return nil
			})
		})
		require.NoError(t, err)
	})

	t.Run("inner work rolls back with the outer transaction", func(t *testing.T) {
		before := count()
		err := trxContext.Execute(t.Context(), func(outer context.Context) error {
			err := trxContext.Execute(outer, func(inner context.Context) error {
				_, err := inner.Value(SQLTransactionKey).(*sql.Tx).Exec("INSERT INTO test_table (value) VALUES ('inner')")
				return err
			})
			require.NoError(t, err)
			return errors.New("outer failed")
		})
		require.Error(t, err)
		assert.Equal(t, before, count())
	})

	t.Run("inner error is returned without ending the outer transaction", func(t *testing.T) {
		before := count()
		innerErr := errors.New("inner failed")
		err := trxContext.Execute(t.Context(), func(outer context.Context) error {
			err := trxContext.Execute(outer, func(context.Context) error { return innerErr })
			assert.ErrorIs(t, err, innerErr)
			// the outer transaction is still usable
			_, err = outer.Value(SQLTransactionKey).(*sql.Tx).Exec("INSERT INTO test_table (value) VALUES ('outer')")
			return err
		})
		require.NoError(t, err)
		assert.Equal(t, before+1, count())
	})

	t.Run("nested calls use one connection", func(t *testing.T) {
		db, err := sql.Open("postgres", testDSN)
		require.NoError(t, err)
		defer db.Close()
		db.SetMaxOpenConns(1)
		single := NewDBTransactionContext(db)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		err = single.Execute(ctx, func(outer context.Context) error {
			return single.Execute(outer, func(context.Context) error { return nil })
		})
		require.NoError(t, err)
	})
}
