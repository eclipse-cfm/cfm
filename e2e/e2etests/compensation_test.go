// Copyright (c) 2025 Metaform Systems, Inc
//
// This program and the accompanying materials are made available under the
// terms of the Apache License, Version 2.0 which is available at
// https://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0
//
// Contributors:
//
//	Metaform Systems, Inc. - initial API and implementation
package e2etests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/natsfixtures"
	"github.com/metaform/connector-fabric-manager/common/sqlstore"
	"github.com/metaform/connector-fabric-manager/e2e/e2efixtures"
	testLauncher "github.com/metaform/connector-fabric-manager/e2e/testagent/launcher"
	papi "github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/metaform/connector-fabric-manager/tmanager/api"
	"github.com/metaform/connector-fabric-manager/tmanager/model/v1alpha1"
	"github.com/stretchr/testify/require"
)

// Test_VerifyAutoCompensation asserts that an orchestration, that hast at least one terminally failed agent, is "auto-compensated".
// This means that the corresponding dispose orchestration is automatically started, rolling back all changes
func Test_VerifyAutoCompensation(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, cfmBucket)

	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)
	defer cleanup()

	pg, dsn, err := sqlstore.SetupTestContainer(t)
	require.NoError(t, err)
	defer pg.Terminate(context.Background())

	_, client := launchPlatform(t, nt.URI, dsn)

	// launch a test agent that always fails -> triggers auto-compensation
	go func() {
		testLauncher.LaunchWithCallback(ctx.Done(), func(ctx papi.ActivityContext) papi.ActivityResult {
			return papi.ActivityResult{
				Result:           papi.ActivityResultFatalError,
				WaitOnReschedule: 0,
				Error:            fmt.Errorf("test error"),
			}
		})
	}()

	waitPManager(t, client)

	err = e2efixtures.CreateTestActivityDefinition(client)
	require.NoError(t, err)

	err = e2efixtures.CreateTestOrchestrationDefinitions(client)
	require.NoError(t, err)

	cell, err := e2efixtures.CreateCell(client)
	require.NoError(t, err)

	dProfile, err := e2efixtures.CreateDataspaceProfile(client)
	require.NoError(t, err)

	deployment := v1alpha1.NewDataspaceProfileDeployment{
		ProfileID: dProfile.ID,
		CellID:    cell.ID,
	}
	err = e2efixtures.DeployDataspaceProfile(deployment, client)
	require.NoError(t, err)

	tenant, err := e2efixtures.CreateTenant(client, map[string]any{})
	require.NoError(t, err)

	newParticipantProfile := v1alpha1.NewParticipantProfileDeployment{
		Identifier:       "did:web:foo.com",
		ParticipantRoles: map[string][]string{dProfile.ID: {e2efixtures.OEMRole}},
		VPAProperties:    map[string]map[string]any{string(model.ConnectorType): {"connectorkey": "connectorvalue"}},
	}

	var participantProfile v1alpha1.ParticipantProfile
	err = client.PostToTManagerWithResponse(fmt.Sprintf("tenants/%s/participant-profiles", tenant.ID), newParticipantProfile, &participantProfile)
	require.NoError(t, err)

	// wait until orchestration have been instantiated and transition to Errored
	var orchestrations []papi.OrchestrationEntry
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		err = client.PostToPManagerWithResponse("orchestrations/query", model.None(), &orchestrations)
		require.NoError(t, err)
		if orchestrations != nil && len(orchestrations) == 1 {
			if orchestrations[0].State == papi.OrchestrationStateErrored {
				break
			}
		}
	}

	require.Len(t, orchestrations, 1, "Expected 1 orchestrations to be present")
	o := orchestrations[0]
	require.Equal(t, papi.OrchestrationStateErrored, o.State)

	// Verify all VPAs are errored out
	var statusProfile v1alpha1.ParticipantProfile
	deployCount := 0
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		err = client.GetTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s", tenant.ID, participantProfile.ID), &statusProfile)
		require.NoError(t, err)
		for _, vpa := range statusProfile.VPAs {
			if vpa.State == api.DeploymentStateError.String() {
				deployCount++
			}
		}
		if deployCount == 3 {
			break
		}
	}
	require.Equal(t, 3, deployCount, "Expected 3 VPAs to be errored out")

	// verify that the compensation orchestration was started
}
