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

package e2etests

import (
	"context"
	"fmt"
	"testing"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/natsfixtures"
	"github.com/metaform/connector-fabric-manager/common/sqlstore"
	"github.com/metaform/connector-fabric-manager/e2e/e2efixtures"
	"github.com/metaform/connector-fabric-manager/tmanager/model/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_VerifyTenantOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, cfmBucket)
	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)
	defer cleanup()

	pg, dsn, err := sqlstore.SetupTestContainer(t)
	require.NoError(t, err)
	defer pg.Terminate(context.Background())

	client := launchPlatformWithAgent(t, nt.URI, dsn)

	waitTManager(t, client)

	verifyTenantGetEndpoints(t, err, client)
	verifyTenantDelete(t, err, client)
	verifyTenantQueries(t, err, client)
	verifyTenantPatch(t, err, client)
}

func verifyTenantDelete(t *testing.T, err error, client *e2efixtures.ApiClient) {
	tenant, err := e2efixtures.CreateTenant(client, map[string]any{"group": "delete"})
	require.NoError(t, err)

	var result []v1alpha1.Tenant
	err = client.GetTManager("tenants", &result)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result))

	err = client.DeleteToTManager(fmt.Sprintf("tenants/%s", tenant.ID))
	require.NoError(t, err)

	result = nil
	err = client.GetTManager("tenants", &result)
	require.NoError(t, err)
	assert.Equal(t, 0, len(result))
}

func verifyTenantGetEndpoints(t *testing.T, err error, client *e2efixtures.ApiClient) {
	tenant, err := e2efixtures.CreateTenant(client, map[string]any{"group": "suppliers"})
	require.NoError(t, err)
	var result []v1alpha1.Tenant
	err = client.GetTManager("tenants", &result)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result))

	var tenantResult v1alpha1.Tenant
	err = client.GetTManager(fmt.Sprintf("tenants/%s", tenant.ID), &tenantResult)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, tenantResult.ID)

	err = client.DeleteToTManager(fmt.Sprintf("tenants/%s", tenant.ID))
	require.NoError(t, err)
}

func verifyTenantQueries(t *testing.T, err error, client *e2efixtures.ApiClient) {
	tenant1, err := e2efixtures.CreateTenant(client, map[string]any{"group": "suppliers"})
	require.NoError(t, err)
	tenant2, err := e2efixtures.CreateTenant(client, map[string]any{"group": "manufacturers"})
	require.NoError(t, err)

	var result []v1alpha1.Tenant
	err = client.PostToTManagerWithResponse("tenants/query", model.Query{Predicate: "properties.group = 'suppliers'"}, &result)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result))

	result = nil
	err = client.PostToTManagerWithResponse("tenants/query", model.Query{Predicate: "true"}, &result)
	require.NoError(t, err)
	assert.Equal(t, 2, len(result))

	result = nil
	err = client.PostToTManagerWithResponse("tenants/query", model.Query{Predicate: "properties.group = 'suppliers' OR properties.group = 'manufacturers'"}, &result)
	require.NoError(t, err)
	assert.Equal(t, 2, len(result))

	err = client.DeleteToTManager(fmt.Sprintf("tenants/%s", tenant1.ID))
	require.NoError(t, err)
	err = client.DeleteToTManager(fmt.Sprintf("tenants/%s", tenant2.ID))
	require.NoError(t, err)
}

func verifyTenantPatch(t *testing.T, err error, client *e2efixtures.ApiClient) {
	tenant1, err := e2efixtures.CreateTenant(client, map[string]any{"group": "suppliers"})
	require.NoError(t, err)
	tenant2, err := e2efixtures.CreateTenant(client, map[string]any{"group": "patch"})
	require.NoError(t, err)

	var result []v1alpha1.Tenant
	err = client.PostToTManagerWithResponse("tenants/query", model.Query{Predicate: "properties.group = 'patch'"}, &result)

	diff := &v1alpha1.TenantPropertiesDiff{
		Properties: map[string]any{"customer": "gold"},
		Removed:    []string{"group"},
	}
	err = client.PatchToTManager(fmt.Sprintf("tenants/%s", result[0].ID), diff)
	require.NoError(t, err)

	result = nil
	err = client.PostToTManagerWithResponse("tenants/query", model.Query{Predicate: "properties.customer = 'gold'"}, &result)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, 1, len(result[0].Properties))
	assert.Equal(t, "gold", result[0].Properties["customer"])

	err = client.DeleteToTManager(fmt.Sprintf("tenants/%s", tenant1.ID))
	require.NoError(t, err)
	err = client.DeleteToTManager(fmt.Sprintf("tenants/%s", tenant2.ID))
	require.NoError(t, err)
}
