/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package e2etests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/e2e/e2efixtures"
	"github.com/eclipse-cfm/cfm/tmanager/model/v1alpha1"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// this test verifies all operations executed on a participant profile

func Test_ParticipantProfileOperations(t *testing.T) {
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
	require.NotNil(t, client)

	waitPManager(t, client)
	waitTManager(t, client)

	tenant, err := e2efixtures.CreateTenant(client, map[string]any{"label": "rotate-key-test"})
	require.NoError(t, err)

	cell, err := e2efixtures.CreateCell(client)
	require.NoError(t, err)

	dataspaceProfile, err := e2efixtures.CreateDataspaceProfile(client)
	require.NoError(t, err)

	deployment := v1alpha1.NewDataspaceProfileDeployment{
		ProfileID: dataspaceProfile.ID,
		CellID:    cell.ID,
	}
	err = e2efixtures.DeployDataspaceProfile(deployment, client)
	require.NoError(t, err)

	newParticipantProfile := v1alpha1.NewParticipantProfileDeployment{
		CellID:           cell.ID,
		Identifier:       "did:web:foo.com",
		ParticipantRoles: map[string][]string{dataspaceProfile.ID: {"test-participant"}},
		Properties: map[string]any{
			model.VPAStateData: map[string]any{
				"participantContextID": "test-participant-context-id",
			},
		},
	}

	var participantProfile v1alpha1.ParticipantProfile
	err = client.PostToTManagerWithResponse(fmt.Sprintf("tenants/%s/participant-profiles", tenant.ID), newParticipantProfile, &participantProfile)
	require.NoError(t, err)
	require.NotNil(t, participantProfile)
	require.NotNil(t, participantProfile.ID)

	t.Run("rotate key success", func(t *testing.T) {
		verifyRotateKey(t, client, tenant, participantProfile)
	})
	t.Run("rotate key invalid request", func(t *testing.T) {
		verifyRotateKeyInvalidRequest(t, client, tenant, participantProfile)
	})

	t.Run("profile not found", func(t *testing.T) {
		err := client.PostToTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s/rotate-keys", tenant.ID, "invalid-profile-id"), v1alpha1.KeyRotationRequest{
			KeyPairID: uuid.NewString(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "Not Found")
	})

	t.Run("tenant does not match", func(t *testing.T) {
		err := client.PostToTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s/rotate-keys", "invalid-tenant-id", participantProfile.ID), v1alpha1.KeyRotationRequest{
			KeyPairID: uuid.NewString(),
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "Bad Request")
	})
}

func verifyRotateKeyInvalidRequest(t *testing.T, client *e2efixtures.ApiClient, tenant *v1alpha1.Tenant, profile v1alpha1.ParticipantProfile) {
	krReq := v1alpha1.KeyRotationRequest{
		// missing: key-id
	}
	jsonData, msErr := json.Marshal(krReq)
	require.NoError(t, msErr)
	fmt.Println(string(jsonData))
	t.Run("missing key-id", func(t *testing.T) {
		err := client.PostToTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s/rotate-keys", tenant.ID, profile.ID), krReq)
		require.Error(t, err)
		require.ErrorContains(t, err, "Bad Request")
	})

	t.Run("missing payload", func(t *testing.T) {
		err := client.PostToTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s/rotate-keys", tenant.ID, profile.ID), nil)
		require.Error(t, err)
		require.ErrorContains(t, err, "Bad Request")
	})
}

func verifyRotateKey(t *testing.T, client *e2efixtures.ApiClient, tenant *v1alpha1.Tenant, profile v1alpha1.ParticipantProfile) {
	krReq := v1alpha1.KeyRotationRequest{
		KeyPairID: "test-key-id",
	}
	jsonData, msErr := json.Marshal(krReq)
	require.NoError(t, msErr)
	fmt.Println(string(jsonData))
	err := client.PostToTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s/rotate-keys", tenant.ID, profile.ID), krReq)
	require.NoError(t, err)
}
