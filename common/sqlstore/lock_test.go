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
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLockTestStore(t *testing.T) *PostgresEntityStore[*testEntity] {
	setupEntityTable(t)
	t.Cleanup(func() { CleanupTestData(t, testDB) })
	_, err := testDB.Exec("INSERT INTO test_entities (id, value) VALUES ('entity-1', 'value')")
	require.NoError(t, err)
	columnNames := []string{"id", "value", "version", "created_at", "metadata"}
	return NewPostgresEntityStore("test_entities", columnNames, recordToEntity, entityToRecord, *createBuilder())
}

// beginWithLock starts a transaction that locks entity-1 in the given mode, and returns its context and a function
// that ends it.
func beginWithLock(t *testing.T, estore *PostgresEntityStore[*testEntity], mode store.LockMode) (context.Context, func()) {
	tx, err := testDB.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	txCtx := context.WithValue(t.Context(), SQLTransactionKey, tx)
	entity, err := estore.FindByIDAndLock(txCtx, "entity-1", mode)
	require.NoError(t, err)
	assert.Equal(t, "value", entity.Value)
	return txCtx, func() { _ = tx.Commit() }
}

// lockWithTimeout tries to lock entity-1 in a new transaction and reports whether that succeeded before the timeout.
func lockWithTimeout(t *testing.T, estore *PostgresEntityStore[*testEntity], mode store.LockMode, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	return NewDBTransactionContext(testDB).Execute(ctx, func(ctx context.Context) error {
		_, err := estore.FindByIDAndLock(ctx, "entity-1", mode)
		return err
	})
}

func TestFindByIDAndLock_SharedLocksDontBlockEachOther(t *testing.T) {
	estore := newLockTestStore(t)
	_, end := beginWithLock(t, estore, store.LockShared)
	defer end()

	require.NoError(t, lockWithTimeout(t, estore, store.LockShared, 2*time.Second))
}

func TestFindByIDAndLock_SharedLockBlocksExclusiveLock(t *testing.T) {
	estore := newLockTestStore(t)
	_, end := beginWithLock(t, estore, store.LockShared)

	require.Error(t, lockWithTimeout(t, estore, store.LockExclusive, 300*time.Millisecond))

	end()
	require.NoError(t, lockWithTimeout(t, estore, store.LockExclusive, 2*time.Second))
}

func TestFindByIDAndLock_ExclusiveLockBlocksSharedLock(t *testing.T) {
	estore := newLockTestStore(t)
	_, end := beginWithLock(t, estore, store.LockExclusive)

	require.Error(t, lockWithTimeout(t, estore, store.LockShared, 300*time.Millisecond))

	end()
	require.NoError(t, lockWithTimeout(t, estore, store.LockShared, 2*time.Second))
}

func TestFindByIDAndLock_DeletedWhileWaiting(t *testing.T) {
	estore := newLockTestStore(t)
	txCtx, end := beginWithLock(t, estore, store.LockExclusive)
	require.NoError(t, estore.Delete(txCtx, "entity-1"))

	result := make(chan error, 1)
	go func() { result <- lockWithTimeout(t, estore, store.LockShared, 5*time.Second) }()
	time.Sleep(200 * time.Millisecond)
	end()

	assert.ErrorIs(t, <-result, types.ErrNotFound)
}

func TestFindByIDAndLock_NotFound(t *testing.T) {
	estore := newLockTestStore(t)

	err := NewDBTransactionContext(testDB).Execute(t.Context(), func(ctx context.Context) error {
		_, err := estore.FindByIDAndLock(ctx, "missing", store.LockExclusive)
		return err
	})

	assert.ErrorIs(t, err, types.ErrNotFound)
}
