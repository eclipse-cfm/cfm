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
)

type sqlTransactionKeyType struct{}

// SQLTransactionKey defines the key for obtaining the transaction from the context.
var SQLTransactionKey = sqlTransactionKeyType{}

type SQLTransactionContext struct {
	db *sql.DB
}

func NewDBTransactionContext(db *sql.DB) *SQLTransactionContext {
	return &SQLTransactionContext{db: db}
}

// Execute runs the operation in a transaction. If the context already carries a transaction, the operation joins it:
// it runs on the same connection, and its error is returned to the caller without committing or rolling back, which is
// left to the outermost Execute. A nested call therefore never needs a second connection, so callers holding a
// transaction can't deadlock on a bounded pool, and nested work commits or rolls back together with the outer work.
func (trxContext *SQLTransactionContext) Execute(ctx context.Context, operation func(context.Context) error) error {
	if _, ok := ctx.Value(SQLTransactionKey).(*sql.Tx); ok {
		return operation(ctx)
	}

	// begin transaction
	tx, err := trxContext.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// rollback on panic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // re-throw panic
		}
	}()

	// execute the operation
	opCtx := context.WithValue(ctx, SQLTransactionKey, tx)
	if err := operation(opCtx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("operation failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}

	// commit if no errors
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
