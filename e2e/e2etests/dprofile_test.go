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

	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/e2e/e2efixtures"
	"github.com/eclipse-cfm/cfm/tmanager/model/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_VerifyDataspaceProfileOperations(t *testing.T) {
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

	verifyDataspaceProfileGetAll(t, err, client)
	verifyDataspaceProfileDelete(t, err, client)
}

func verifyDataspaceProfileDelete(t *testing.T, err error, client *e2efixtures.ApiClient) {
	profile, err := e2efixtures.CreateDataspaceProfile(client)
	require.NoError(t, err)

	var result []v1alpha1.DataspaceProfile
	err = client.GetTManager("dataspace-profiles", &result)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result))

	err = client.DeleteToTManager(fmt.Sprintf("dataspace-profiles/%s", profile.ID))
	require.NoError(t, err)

	result = nil
	err = client.GetTManager("dataspace-profiles", &result)
	require.NoError(t, err)
	assert.Equal(t, 0, len(result))
}

func verifyDataspaceProfileGetAll(t *testing.T, err error, client *e2efixtures.ApiClient) {
	profile, err := e2efixtures.CreateDataspaceProfile(client)
	require.NoError(t, err)
	var result []v1alpha1.Tenant
	err = client.GetTManager("dataspace-profiles", &result)
	require.NoError(t, err)
	require.NoError(t, err)
	assert.Equal(t, 1, len(result))

	err = client.DeleteToTManager(fmt.Sprintf("dataspace-profiles/%s", profile.ID))
	require.NoError(t, err)
}
