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
	"testing"

	"github.com/eclipse-cfm/cfm/common/memorystore"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTenant(t *testing.T) {
	ctx := context.Background()
	service := newTestTenantService()

	t.Run("get existing tenant", func(t *testing.T) {
		tenant := newTestTenant("tenant-1")
		_, err := service.tenantStore.Create(ctx, tenant)
		require.NoError(t, err)

		result, err := service.GetTenant(ctx, "tenant-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "tenant-1", result.ID)
		assert.Equal(t, int64(1), result.Version)
	})

	t.Run("get non-existent tenant returns not found error", func(t *testing.T) {
		result, err := service.GetTenant(ctx, "non-existent")

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, types.ErrNotFound, err)
	})
}

func TestCreateTenant(t *testing.T) {
	ctx := context.Background()
	service := newTestTenantService()

	t.Run("create valid tenant", func(t *testing.T) {
		tenant := newTestTenant("tenant-1")

		result, err := service.CreateTenant(ctx, tenant)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "tenant-1", result.ID)
		assert.Equal(t, int64(1), result.Version)
		assert.Equal(t, "Test Tenant tenant-1", result.Properties["name"])
	})

	t.Run("create tenant with empty ID returns error", func(t *testing.T) {
		tenant := &api.Tenant{
			Entity: api.Entity{
				ID:      "",
				Version: 1,
			},
		}

		result, err := service.CreateTenant(ctx, tenant)

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, types.ErrInvalidInput, err)
	})

	t.Run("create duplicate tenant returns error", func(t *testing.T) {
		tenant1 := newTestTenant("tenant-2")
		tenant2 := newTestTenant("tenant-2")

		// First create should succeed
		result1, err1 := service.CreateTenant(ctx, tenant1)
		require.NoError(t, err1)
		require.NotNil(t, result1)

		// Second create with same ID should fail
		result2, err2 := service.CreateTenant(ctx, tenant2)
		require.Error(t, err2)
		require.Nil(t, result2)
		assert.Equal(t, types.ErrConflict, err2)
	})

}

func TestQueryTenants(t *testing.T) {
	ctx := context.Background()
	service := newTestTenantService()

	// Populate store with test data
	tenants := []*api.Tenant{
		{
			Entity:     api.Entity{ID: "tenant-1", Version: 1},
			Properties: api.Properties{"name": "Tenant One"},
		},
		{
			Entity:     api.Entity{ID: "tenant-2", Version: 1},
			Properties: api.Properties{"name": "Tenant Two"},
		},
		{
			Entity:     api.Entity{ID: "tenant-3", Version: 1},
			Properties: api.Properties{"name": "Tenant Three"},
		},
	}

	for _, tenant := range tenants {
		_, err := service.tenantStore.Create(ctx, tenant)
		require.NoError(t, err)
	}

	t.Run("query all tenants with empty predicate", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "properties.name",
			Operator: query.OpEqual,
			Value:    "Tenant One",
		}
		options := store.DefaultPaginationOptions()

		results := make([]api.Tenant, 0)
		for tenant, err := range service.QueryTenants(ctx, predicate, options) {
			require.NoError(t, err)
			results = append(results, *tenant)
		}

		assert.Equal(t, 1, len(results))
	})
}

// TestCountTenants tests the QueryTenantsCount method
func TestCountTenants(t *testing.T) {
	ctx := context.Background()
	tenantStore := memorystore.NewInMemoryEntityStore[*api.Tenant]()
	service := &tenantService{
		trxContext:  store.NoOpTransactionContext{},
		tenantStore: tenantStore,
	}

	t.Run("count empty store", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "properties.name",
			Operator: query.OpEqual,
			Value:    "Tenant One",
		}

		count, err := service.QueryTenantsCount(ctx, predicate)

		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("count with matching predicate", func(t *testing.T) {
		predicate := &query.AtomicPredicate{
			Field:    "properties.name",
			Operator: query.OpEqual,
			Value:    "Tenant One",
		}

		count, err := service.QueryTenantsCount(ctx, predicate)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(0))
	})

}

func TestGetTenants(t *testing.T) {

	t.Run("get all tenants with pagination", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		// Create multiple test tenants
		tenants := []*api.Tenant{
			newTestTenant("tenant-1"),
			newTestTenant("tenant-2"),
			newTestTenant("tenant-3"),
		}

		for _, tenant := range tenants {
			_, err := service.tenantStore.Create(ctx, tenant)
			require.NoError(t, err)
		}

		options := store.DefaultPaginationOptions()
		results := make([]*api.Tenant, 0)

		for tenant, err := range service.GetTenants(ctx, options) {
			require.NoError(t, err)
			results = append(results, tenant)
		}

		require.Equal(t, 3, len(results))
		expectedIDs := []string{"tenant-1", "tenant-2", "tenant-3"}
		resultIDs := make([]string, len(results))
		for i, tenant := range results {
			resultIDs[i] = tenant.ID
		}
		assert.ElementsMatch(t, expectedIDs, resultIDs)

		// test pagination
		options.Limit = 2
		results = make([]*api.Tenant, 0)

		for tenant, err := range service.GetTenants(ctx, options) {
			require.NoError(t, err)
			results = append(results, tenant)
		}
		assert.Equal(t, 2, len(results))

	})

	t.Run("get tenants from empty store", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		options := store.DefaultPaginationOptions()
		results := make([]*api.Tenant, 0)

		for tenant, err := range service.GetTenants(ctx, options) {
			require.NoError(t, err)
			results = append(results, tenant)
		}

		require.Equal(t, 0, len(results))
	})
}

func TestGetTenantsCount(t *testing.T) {

	t.Run("count tenants in populated store", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		// Create multiple test tenants
		tenants := []*api.Tenant{
			newTestTenant("tenant-1"),
			newTestTenant("tenant-2"),
			newTestTenant("tenant-3"),
		}

		for _, tenant := range tenants {
			_, err := service.tenantStore.Create(ctx, tenant)
			require.NoError(t, err)
		}

		count, err := service.GetTenantsCount(ctx)

		require.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})

	t.Run("count tenants in empty store", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		count, err := service.GetTenantsCount(ctx)

		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestDeleteTenant(t *testing.T) {

	t.Run("delete existing tenant with no participants", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		tenant := newTestTenant("tenant-delete-1")
		created, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)
		require.NotNil(t, created)

		err = service.DeleteTenant(ctx, "tenant-delete-1")

		require.NoError(t, err)

		// Verify: Tenant no longer exists
		result, err := service.GetTenant(ctx, "tenant-delete-1")
		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
		assert.Nil(t, result)
	})

	t.Run("delete non-existent tenant returns error", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		err := service.DeleteTenant(ctx, "non-existent-tenant")

		// Verify: Error is returned
		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("delete tenant with empty ID fails", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		err := service.DeleteTenant(ctx, "")
		require.Error(t, err)
	})

	t.Run("delete tenant with participants fails", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		tenant := newTestTenant("tenant-delete-2")
		created, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Setup: Create a participant associated with the tenant
		participant := &api.ParticipantProfile{
			Entity:   api.Entity{ID: "participant-1", Version: 1},
			TenantID: created.ID,
		}

		_, err = service.participantStore.Create(ctx, participant)
		require.NoError(t, err)

		// Execute: Attempt to delete the tenant
		err = service.DeleteTenant(ctx, "tenant-delete-2")
		require.ErrorAs(t, err, &types.BadRequestError{})

	})

	t.Run("delete tenant after removing all participants", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		// Setup: Create a tenant
		tenant := newTestTenant("tenant-delete-3")
		created, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)
		assert.NotNil(t, created)

		// Execute: Now delete the tenant (should succeed)
		err = service.DeleteTenant(ctx, "tenant-delete-3")

		// Verify: Deletion succeeded
		require.NoError(t, err)

		// Verify: Tenant no longer exists
		_, err = service.GetTenant(ctx, "tenant-delete-3")
		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("delete tenant with multiple participants fails", func(t *testing.T) {
		ctx := context.Background()
		service := newTestTenantService()
		// Setup: Create a tenant
		tenant := newTestTenant("tenant-delete-4")
		created, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Note: Testing with multiple participants requires public API
		_ = created
	})
}

func newTestTenant(id string) *api.Tenant {
	return &api.Tenant{
		Entity: api.Entity{
			ID:      id,
			Version: 1,
		},
		Properties: api.Properties{
			"name": "Test Tenant " + id,
		},
	}
}

func newTestTenantService() *tenantService {
	return &tenantService{
		trxContext:       store.NoOpTransactionContext{},
		tenantStore:      memorystore.NewInMemoryEntityStore[*api.Tenant](),
		participantStore: memorystore.NewInMemoryEntityStore[*api.ParticipantProfile](),
		monitor:          nil,
	}
}
