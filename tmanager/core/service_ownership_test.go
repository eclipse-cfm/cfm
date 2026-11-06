//  Copyright (c) 2026 Metaform Systems, Inc
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

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func ownerContext(ctx context.Context, subject string) context.Context {
	return api.ContextWithTenantAccess(ctx, api.OwnedTenantsAccess(subject))
}

// newOwnedTenants creates a tenant owned by client-a, one owned by client-b and one without an owner, as created
// before owners were recorded.
func newOwnedTenants(t *testing.T, tenantStore store.EntityStore[*api.Tenant]) {
	for id, owner := range map[string]string{"tenant-a": "client-a", "tenant-b": "client-b", "tenant-legacy": ""} {
		tenant := newTestTenant(id)
		tenant.Owner = owner
		_, err := tenantStore.Create(t.Context(), tenant)
		require.NoError(t, err)
	}
}

func TestCreateTenant_RecordsOwner(t *testing.T) {
	service := newTestTenantService()

	created, err := service.CreateTenant(ownerContext(t.Context(), "client-a"), newTestTenant("tenant-1"))

	require.NoError(t, err)
	assert.Equal(t, "client-a", created.Owner)
	stored, err := service.tenantStore.FindByID(t.Context(), "tenant-1")
	require.NoError(t, err)
	assert.Equal(t, "client-a", stored.Owner)
}

func TestCreateTenant_AdminIsOwner(t *testing.T) {
	service := newTestTenantService()
	ctx := api.ContextWithTenantAccess(t.Context(), api.AllTenantsAccess("operator"))

	created, err := service.CreateTenant(ctx, newTestTenant("tenant-1"))

	require.NoError(t, err)
	assert.Equal(t, "operator", created.Owner)
}

func TestTenantService_RequiresAccess(t *testing.T) {
	service := newTestTenantService()
	newOwnedTenants(t, service.tenantStore)
	ctx := t.Context()

	_, err := service.GetTenant(ctx, "tenant-a")
	assert.True(t, types.IsFatal(err))

	_, err = service.CreateTenant(ctx, newTestTenant("tenant-1"))
	assert.True(t, types.IsFatal(err))

	_, err = service.GetTenantsCount(ctx)
	assert.True(t, types.IsFatal(err))

	_, err = collection.CollectAll(service.GetTenants(ctx, store.DefaultPaginationOptions()))
	assert.True(t, types.IsFatal(err))
}

func TestTenantService_OwnerSeesOnlyOwnTenants(t *testing.T) {
	service := newTestTenantService()
	newOwnedTenants(t, service.tenantStore)
	ctx := ownerContext(t.Context(), "client-a")

	t.Run("get", func(t *testing.T) {
		tenant, err := service.GetTenant(ctx, "tenant-a")
		require.NoError(t, err)
		assert.Equal(t, "tenant-a", tenant.ID)

		for _, id := range []string{"tenant-b", "tenant-legacy"} {
			_, err := service.GetTenant(ctx, id)
			assert.ErrorIs(t, err, types.ErrNotFound, id)
		}
	})

	t.Run("patch", func(t *testing.T) {
		err := service.PatchTenant(ctx, "tenant-b", map[string]any{"key": "value"}, nil)
		assert.ErrorIs(t, err, types.ErrNotFound)

		stored, err := service.tenantStore.FindByID(t.Context(), "tenant-b")
		require.NoError(t, err)
		assert.NotContains(t, stored.Properties, "key")
	})

	t.Run("delete", func(t *testing.T) {
		err := service.DeleteTenant(ctx, "tenant-b")
		assert.ErrorIs(t, err, types.ErrNotFound)

		_, err = service.tenantStore.FindByID(t.Context(), "tenant-b")
		assert.NoError(t, err)
	})

	t.Run("list", func(t *testing.T) {
		tenants, err := collection.CollectAll(service.GetTenants(ctx, store.DefaultPaginationOptions()))
		require.NoError(t, err)
		require.Len(t, tenants, 1)
		assert.Equal(t, "tenant-a", tenants[0].ID)

		count, err := service.GetTenantsCount(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("query", func(t *testing.T) {
		// The caller's predicate cannot widen the result to other clients' tenants.
		predicate := query.Or(query.Eq("id", "tenant-b"), query.Eq("id", "tenant-a"))

		tenants, err := collection.CollectAll(service.QueryTenants(ctx, predicate, store.DefaultPaginationOptions()))
		require.NoError(t, err)
		require.Len(t, tenants, 1)
		assert.Equal(t, "tenant-a", tenants[0].ID)

		count, err := service.QueryTenantsCount(ctx, predicate)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

func TestTenantService_AdminSeesAllTenants(t *testing.T) {
	service := newTestTenantService()
	newOwnedTenants(t, service.tenantStore)
	ctx := api.ContextWithTenantAccess(t.Context(), api.AllTenantsAccess("operator"))

	for _, id := range []string{"tenant-a", "tenant-b", "tenant-legacy"} {
		_, err := service.GetTenant(ctx, id)
		assert.NoError(t, err, id)
	}

	count, err := service.GetTenantsCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestParticipantService_Ownership(t *testing.T) {
	newService := func(t *testing.T) *participantService {
		service := newDeployableTestService(t)
		newOwnedTenants(t, service.tenantStore)
		for _, tenantID := range []string{"tenant-a", "tenant-b"} {
			profile := newTestParticipantProfile(tenantID, "participant-"+tenantID)
			profile.Owner = "client-" + tenantID[len("tenant-"):]
			_, err := service.participantStore.Create(t.Context(), profile)
			require.NoError(t, err)
		}
		return service
	}

	t.Run("deploy to an own tenant records the owner", func(t *testing.T) {
		service := newService(t)
		ctx := ownerContext(t.Context(), "client-a")
		mockClient := new(mockProvisionClient)
		mockClient.On("Send", ctx, mock.Anything).Return(nil)
		service.provisionClient = mockClient

		profile, err := service.DeployProfile(ctx, "tenant-a", &api.NewParticipantProfileDeployment{Identifier: "participant-identifier"})

		require.NoError(t, err)
		assert.Equal(t, "client-a", profile.Owner)
		stored, err := service.participantStore.FindByID(t.Context(), profile.ID)
		require.NoError(t, err)
		assert.Equal(t, "client-a", stored.Owner)
	})

	t.Run("deploy to another client's tenant is not found", func(t *testing.T) {
		service := newService(t)
		ctx := ownerContext(t.Context(), "client-a")
		mockClient := new(mockProvisionClient)
		service.provisionClient = mockClient

		_, err := service.DeployProfile(ctx, "tenant-b", &api.NewParticipantProfileDeployment{Identifier: "participant-identifier"})

		require.ErrorIs(t, err, types.ErrNotFound)
		mockClient.AssertNotCalled(t, "Send", mock.Anything, mock.Anything)
	})

	t.Run("get and dispose another client's profile are not found", func(t *testing.T) {
		service := newService(t)
		ctx := ownerContext(t.Context(), "client-a")
		mockClient := new(mockProvisionClient)
		service.provisionClient = mockClient

		_, err := service.GetProfile(ctx, "tenant-b", "participant-tenant-b")
		assert.ErrorIs(t, err, types.ErrNotFound)

		err = service.DisposeProfile(ctx, "tenant-b", "participant-tenant-b")
		assert.ErrorIs(t, err, types.ErrNotFound)
		mockClient.AssertNotCalled(t, "Send", mock.Anything, mock.Anything)

		profile, err := service.GetProfile(ctx, "tenant-a", "participant-tenant-a")
		require.NoError(t, err)
		assert.Equal(t, "participant-tenant-a", profile.ID)
	})

	t.Run("query returns only own profiles", func(t *testing.T) {
		service := newService(t)
		ctx := ownerContext(t.Context(), "client-a")

		profiles, err := collection.CollectAll(service.QueryProfiles(ctx, &query.MatchAllPredicate{}, store.DefaultPaginationOptions()))
		require.NoError(t, err)
		require.Len(t, profiles, 1)
		assert.Equal(t, "participant-tenant-a", profiles[0].ID)

		count, err := service.QueryProfilesCount(ctx, query.Eq("tenantId", "tenant-b"))
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("admin sees all profiles", func(t *testing.T) {
		service := newService(t)
		ctx := api.ContextWithTenantAccess(t.Context(), api.AllTenantsAccess("operator"))

		count, err := service.QueryProfilesCount(ctx, &query.MatchAllPredicate{})
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
}
