// tmanager/sqlstore/tenant_test.go

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
	"strconv"
	"testing"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTenantStore_Creation tests store creation
func TestNewTenantStore_Creation(t *testing.T) {
	estore := newTenantStore()

	require.NotNil(t, estore)
	assert.NotNil(t, estore)
}

// TestNewTenantStore_FindByID_Success tests successful tenant retrieval
func TestNewTenantStore_FindByID_Success(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	tenant := &api.Tenant{
		Entity: api.Entity{
			ID:      "tenant-1",
			Version: 1,
		},
		Properties: map[string]any{
			"name":   "Test Tenant",
			"region": "us-east-1",
			"status": "active",
		},
	}

	record, err := tenantEntityToRecord(tenant)
	require.NoError(t, err)

	propertiesVal := []byte("{}")
	if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
		propertiesVal = propertiesBytes
	}

	_, err = testDB.Exec(
		"INSERT INTO tenants (id, version, properties) VALUES ($1, $2, $3)",
		record.Values["id"],
		record.Values["version"],
		propertiesVal,
	)
	require.NoError(t, err)

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "tenant-1")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "tenant-1", retrieved.ID)
	assert.Equal(t, int64(1), retrieved.Version)
	assert.NotNil(t, retrieved.Properties)
}

// TestNewTenantStore_FindByID_NotFound tests tenant not found
func TestNewTenantStore_FindByID_NotFound(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	_, err = estore.FindByID(txCtx, "non-existent")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewTenantStore_Create tests creating a new tenant
func TestNewTenantStore_Create(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	tenant := &api.Tenant{
		Entity: api.Entity{
			ID:      "new-tenant",
			Version: 1,
		},
		Properties: map[string]any{
			"name":        "New Tenant",
			"region":      "eu-west-1",
			"environment": "staging",
		},
	}

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	created, err := estore.Create(txCtx, tenant)
	require.NoError(t, err)
	assert.Equal(t, "new-tenant", created.ID)
	assert.Equal(t, int64(1), created.Version)
}

// TestNewTenantStore_SearchByPropertiesPredicate_NameEquality tests searching by properties.name
func TestNewTenantStore_SearchByPropertiesPredicate_NameEquality(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	tenants := []*api.Tenant{
		{
			Entity: api.Entity{
				ID:      "tenant-prod-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Production Tenant",
				"region": "us-east-1",
				"tier":   "premium",
			},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-dev-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Development Tenant",
				"region": "us-west-2",
				"tier":   "standard",
			},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-staging-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Production Tenant",
				"region": "eu-west-1",
				"tier":   "standard",
			},
		},
	}

	// Insert test tenants
	for _, tenant := range tenants {
		record, err := tenantEntityToRecord(tenant)
		require.NoError(t, err)

		propertiesVal := []byte("{}")
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO tenants (id, version, properties) VALUES ($1, $2, $3)",
			record.Values["id"],
			record.Values["version"],
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Search for tenants with name "Production Tenant"
	predicate := query.Eq("properties.name", "Production Tenant")

	count := 0
	for tenant, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, tenant)
		count++

		// Verify the tenant has the expected name
		if name, ok := tenant.Properties["name"].(string); ok {
			assert.Equal(t, "Production Tenant", name)
		}
	}

	// Should find 2 tenants with "Production Tenant" name
	assert.Equal(t, 2, count)
}

// TestNewTenantStore_SearchByPropertiesPredicate_RegionEquality tests searching by properties.region
func TestNewTenantStore_SearchByPropertiesPredicate_RegionEquality(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	tenants := []*api.Tenant{
		{
			Entity: api.Entity{
				ID:      "tenant-us-east-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Tenant 1",
				"region": "us-east-1",
			},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-eu-west-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Tenant 2",
				"region": "eu-west-1",
			},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-us-east-2",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Tenant 3",
				"region": "us-east-1",
			},
		},
	}

	// Insert test tenants
	for _, tenant := range tenants {
		record, err := tenantEntityToRecord(tenant)
		require.NoError(t, err)

		propertiesVal := []byte("{}")
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO tenants (id, version, properties) VALUES ($1, $2, $3)",
			record.Values["id"],
			record.Values["version"],
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("properties.region", "us-east-1")

	count := 0
	for tenant, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, tenant)
		count++

		if region, ok := tenant.Properties["region"].(string); ok {
			assert.Equal(t, "us-east-1", region)
		}
	}

	// Should find 2 tenants with "us-east-1" region
	assert.Equal(t, 2, count)
}

// TestNewTenantStore_SearchByPropertiesPredicate_MultipleFields tests searching by multiple property fields
func TestNewTenantStore_SearchByPropertiesPredicate_MultipleFields(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	tenants := []*api.Tenant{
		{
			Entity: api.Entity{
				ID:      "tenant-premium-prod-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Premium Production",
				"tier":   "premium",
				"status": "active",
			},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-standard-prod-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Standard Production",
				"tier":   "standard",
				"status": "active",
			},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-premium-dev-1",
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Premium Development",
				"tier":   "premium",
				"status": "inactive",
			},
		},
	}

	// Insert test tenants
	for _, tenant := range tenants {
		record, err := tenantEntityToRecord(tenant)
		require.NoError(t, err)

		propertiesVal := []byte("{}")
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO tenants (id, version, properties) VALUES ($1, $2, $3)",
			record.Values["id"],
			record.Values["version"],
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Search for premium tier tenants
	predicate := query.Eq("properties.tier", "premium")

	count := 0
	for tenant, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, tenant)
		count++

		if tier, ok := tenant.Properties["tier"].(string); ok {
			assert.Equal(t, "premium", tier)
		}
	}

	// Should find 2 premium tier tenants
	assert.Equal(t, 2, count)
}

// TestNewTenantStore_SearchByPropertiesPredicate_Status tests searching by properties.status
func TestNewTenantStore_SearchByPropertiesPredicate_Status(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	statuses := []string{"active", "inactive", "active", "suspended"}

	for i := 1; i <= 4; i++ {
		tenant := &api.Tenant{
			Entity: api.Entity{
				ID:      "tenant-status-" + strconv.Itoa(i),
				Version: 1,
			},
			Properties: map[string]any{
				"name":   "Tenant " + strconv.Itoa(i),
				"status": statuses[i-1],
			},
		}

		record, err := tenantEntityToRecord(tenant)
		require.NoError(t, err)

		propertiesVal := []byte("{}")
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO tenants (id, version, properties) VALUES ($1, $2, $3)",
			record.Values["id"],
			record.Values["version"],
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("properties.status", "active")

	count := 0
	for tenant, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, tenant)
		count++

		if status, ok := tenant.Properties["status"].(string); ok {
			assert.Equal(t, "active", status)
		}
	}

	// Should find 2 active tenants
	assert.Equal(t, 2, count)
}

// TestNewTenantStore_SearchByIDPredicate tests searching by ID
func TestNewTenantStore_SearchByIDPredicate(t *testing.T) {
	setupTenantTable(t, testDB)
	defer cleanupTenantTestData(t, testDB)

	tenants := []*api.Tenant{
		{
			Entity: api.Entity{
				ID:      "tenant-search-1",
				Version: 1,
			},
			Properties: map[string]any{"name": "Search Test 1"},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-search-2",
				Version: 1,
			},
			Properties: map[string]any{"name": "Search Test 2"},
		},
		{
			Entity: api.Entity{
				ID:      "tenant-other-1",
				Version: 1,
			},
			Properties: map[string]any{"name": "Other Test"},
		},
	}

	// Insert test tenants
	for _, tenant := range tenants {
		record, err := tenantEntityToRecord(tenant)
		require.NoError(t, err)

		propertiesVal := []byte("{}")
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO tenants (id, version, properties) VALUES ($1, $2, $3)",
			record.Values["id"],
			record.Values["version"],
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newTenantStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("id", "tenant-search-1")

	count := 0
	for tenant, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, tenant)
		count++

		assert.Equal(t, "tenant-search-1", tenant.ID)
	}

	assert.Equal(t, 1, count)
}

// cleanupTenantTestData removes test data from the tenants table
func cleanupTenantTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", cfmTenantsTable))
	require.NoError(t, err)
}

// setupTenantTable creates the tenants table for testing
func setupTenantTable(t *testing.T, db *sql.DB) {
	err := createTenantsTable(db)
	require.NoError(t, err)
}
