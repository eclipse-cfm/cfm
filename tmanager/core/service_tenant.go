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

package core

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

type tenantService struct {
	trxContext       store.TransactionContext
	tenantStore      store.EntityStore[*api.Tenant]
	participantStore store.EntityStore[*api.ParticipantProfile]
	monitor          system.LogMonitor
}

func (t tenantService) GetTenant(ctx context.Context, tenantID string) (*api.Tenant, error) {
	return store.Trx[api.Tenant](t.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.Tenant, error) {
		return t.tenantStore.FindByID(ctx, tenantID)
	})
}

func (t tenantService) CreateTenant(ctx context.Context, tenant *api.Tenant) (*api.Tenant, error) {
	return store.Trx[api.Tenant](t.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.Tenant, error) {
		return t.tenantStore.Create(ctx, tenant)
	})
}

func (t tenantService) PatchTenant(ctx context.Context, id string, properties map[string]any, remove []string) error {
	return t.trxContext.Execute(ctx, func(ctx context.Context) error {
		tenant, err := t.tenantStore.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				return err
			}
			return fmt.Errorf("tenant %s not found: %w", id, err)
		}
		for key, value := range properties {
			tenant.Properties[key] = value
		}
		for _, key := range remove {
			delete(tenant.Properties, key)
		}
		err = t.tenantStore.Update(ctx, tenant)
		if err != nil {
			return fmt.Errorf("unable to patch tenant %s: %w", id, err)
		}
		return nil
	})
}

func (t tenantService) DeleteTenant(ctx context.Context, tenantID string) error {
	return t.trxContext.Execute(ctx, func(ctx context.Context) error {
		tenant, err := t.tenantStore.FindByID(ctx, tenantID)
		if err != nil {
			return err
		}

		count, err := t.participantStore.CountByPredicate(ctx, &query.AtomicPredicate{
			Field:    "tenantId",
			Operator: query.OpEqual,
			Value:    tenant.ID,
		})
		if err != nil {
			return err
		}

		if count > 0 {
			return types.NewClientError("cannot delete tenant with participants")
		}

		return t.tenantStore.Delete(ctx, tenant.ID)
	})
}

func (t tenantService) QueryTenants(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.Tenant, error] {
	return t.executeStoreIterator(ctx, func(ctx context.Context) iter.Seq2[*api.Tenant, error] {
		return t.tenantStore.FindByPredicatePaginated(ctx, predicate, options)
	})
}

func (t tenantService) GetTenants(ctx context.Context, options store.PaginationOptions) iter.Seq2[*api.Tenant, error] {
	return t.executeStoreIterator(ctx, func(ctx context.Context) iter.Seq2[*api.Tenant, error] {
		return t.tenantStore.GetAllPaginated(ctx, options)
	})
}

func (t tenantService) GetTenantsCount(ctx context.Context) (int64, error) {
	return t.tenantStore.GetAllCount(ctx)
}

func (t tenantService) QueryTenantsCount(ctx context.Context, predicate query.Predicate) (int64, error) {
	var count int64
	err := t.trxContext.Execute(ctx, func(ctx context.Context) error {
		c, err := t.tenantStore.CountByPredicate(ctx, predicate)
		count = c
		return err
	})
	return count, err
}

// executeStoreIterator wraps store iterator operations in a transaction context
func (t tenantService) executeStoreIterator(ctx context.Context, storeOp func(context.Context) iter.Seq2[*api.Tenant, error]) iter.Seq2[*api.Tenant, error] {
	return func(yield func(*api.Tenant, error) bool) {
		err := t.trxContext.Execute(ctx, func(ctx context.Context) error {
			for tenant, err := range storeOp(ctx) {
				if !yield(tenant, err) {
					return context.Canceled
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			yield(&api.Tenant{}, err)
		}
	}
}
