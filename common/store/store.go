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

package store

import (
	"context"
	"iter"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/system"
)

const (
	TransactionContextKey system.ServiceType = "store:TransactionContext"
)

// TransactionContext defines an interface for managing transactional operations.
type TransactionContext interface {
	Execute(ctx context.Context, callback func(ctx context.Context) error) error
}

// TrxFunc represents a transactional function wrapper allowing execution with a TransactionContext.
// For example:
//
//	store.Trx(ctx).AndReturn(ctx, func(ctx context.Context) (*MyType, error) {
//		return store.FindByID(ctx, "my-id")
//	})
type TrxFunc[T any] struct {
	ctx TransactionContext
}

func Trx[T any](ctx TransactionContext) TrxFunc[T] {
	return TrxFunc[T]{ctx: ctx}
}

func (tf TrxFunc[T]) AndReturn(ctx context.Context, callback func(context.Context) (*T, error)) (*T, error) {
	var result *T
	var callbackErr error

	err := tf.ctx.Execute(ctx, func(ctx context.Context) error {
		result, callbackErr = callback(ctx)
		return callbackErr
	})

	if err != nil {
		return nil, err
	}

	return result, callbackErr
}

type NoOpTransactionContext struct{}

func (n NoOpTransactionContext) Execute(ctx context.Context, callback func(ctx context.Context) error) error {
	return callback(ctx)
}

type NoOpTrxAssembly struct {
	system.DefaultServiceAssembly
}

func (n NoOpTrxAssembly) Name() string {
	return "NoOpTrxAssembly"
}

func (n NoOpTrxAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{TransactionContextKey}
}

func (n *NoOpTrxAssembly) Init(context *system.InitContext) error {
	context.Registry.Register(TransactionContextKey, NoOpTransactionContext{})
	return nil
}

// PaginationOptions defines pagination parameters for entity retrieval.
type PaginationOptions struct {
	// Offset is the number of items to skip from the beginning.
	Offset int64
	// Limit is the maximum number of items to return. If 0, returns all items.
	Limit int64
	// Cursor is an optional cursor for cursor-based pagination (implementation-specific).
	Cursor string
}

// DefaultPaginationOptions returns default pagination settings (no pagination).
func DefaultPaginationOptions() PaginationOptions {
	return PaginationOptions{
		Offset: 0,
		Limit:  0, // 0 means no limit
		Cursor: "",
	}
}

// EntityStore defines the interface for entity storage.
type EntityStore[T EntityType] interface {
	FindByID(ctx context.Context, id string) (T, error)
	Exists(ctx context.Context, id string) (bool, error)
	Create(ctx context.Context, entity T) (T, error)
	Update(ctx context.Context, entity T) error // T is already a pointer type
	Delete(ctx context.Context, id string) error
	GetAll(ctx context.Context) iter.Seq2[T, error]
	GetAllCount(ctx context.Context) (int64, error)
	GetAllPaginated(ctx context.Context, opts PaginationOptions) iter.Seq2[T, error]
	FindByPredicate(ctx context.Context, predicate query.Predicate) iter.Seq2[T, error]
	FindByPredicatePaginated(ctx context.Context, predicate query.Predicate, opts PaginationOptions) iter.Seq2[T, error]
	FindFirstByPredicate(ctx context.Context, predicate query.Predicate) (T, error)
	CountByPredicate(ctx context.Context, predicate query.Predicate) (int64, error)
	DeleteByPredicate(ctx context.Context, predicate query.Predicate) error
}

// EntityType defines a versionable entity.
type EntityType interface {
	GetID() string
	GetVersion() int64
	IncrementVersion()
}
