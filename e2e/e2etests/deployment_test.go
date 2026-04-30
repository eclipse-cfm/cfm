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

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/e2e/e2efixtures"
	papi "github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/eclipse-cfm/cfm/tmanager/model/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_VerifyE2E(t *testing.T) {

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

	var statusProfile v1alpha1.ParticipantProfile

	// Verify all VPAs are active
	deployCount := 0
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		err = client.GetTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s", tenant.ID, participantProfile.ID), &statusProfile)
		require.NoError(t, err)
		for _, vpa := range statusProfile.VPAs {
			if vpa.State == api.DeploymentStateActive.String() {
				deployCount++
			}
		}
		if deployCount == 4 {
			break
		}
	}
	require.Equal(t, 4, deployCount, "Expected 4 deployments to be active")

	var participantProfiles []api.ParticipantProfile
	err = client.PostToTManagerWithResponse(
		"participant-profiles/query",
		model.Query{Predicate: "vpas.type='cfm.connector' AND vpas.properties.connectorkey='connectorvalue'"}, &participantProfiles)
	require.NoError(t, err)
	assert.Equal(t, 1, len(participantProfiles), "Expected 1 profile to be found")

	var pProfileResult []v1alpha1.ParticipantProfile
	err = client.GetTManager(fmt.Sprintf("tenants/%s/participant-profiles", tenant.ID), &pProfileResult)
	require.NoError(t, err)
	assert.Equal(t, 1, len(pProfileResult), "Expected 1 participant profile to be found")

	// Verify round-tripping of VPA properties - these are supplied during profile creation and are added to the VPA
	//
	// Check for VPA that contains a key with "cfm.connector" value and verify it has "connectorkey"
	var connectorVPA *v1alpha1.VirtualParticipantAgent
	for _, vpa := range statusProfile.VPAs {
		if vpa.Type == model.ConnectorType {
			connectorVPA = &vpa
			break
		}
	}
	require.NotNil(t, connectorVPA, "Expected to find a VPA with cfm.connector type")
	require.NotNil(t, connectorVPA.Properties, "Connector VPA properties should not be nil")
	require.Contains(t, connectorVPA.Properties, "connectorkey", "Connector VPA should contain 'connectorkey' property")

	// verify return of agent state data
	stateData := statusProfile.Properties[model.VPAStateData].(map[string]any)
	assert.NotNil(t, 1, stateData)
	assert.Equal(t, "test output", stateData["agent.test.output"])
	assert.True(t, stateData["agent.test.credentials.received"].(bool))

	var orchestrations []papi.OrchestrationEntry
	err = client.PostToPManagerWithResponse(
		"orchestrations/query",
		model.Query{Predicate: fmt.Sprintf("correlationId = '%s'", statusProfile.ID)}, &orchestrations)
	require.NoError(t, err)
	require.Equal(t, 1, len(orchestrations), "Expected 1 orchestration to be created")
	assert.Equal(t, papi.OrchestrationStateCompleted, orchestrations[0].State)

	// Dispose VPAs
	err = client.DeleteToTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s", tenant.ID, participantProfile.ID))
	require.NoError(t, err)

	disposeCount := 0
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		err = client.GetTManager(fmt.Sprintf("tenants/%s/participant-profiles/%s", tenant.ID, participantProfile.ID), &statusProfile)
		require.NoError(t, err)
		for _, vpa := range statusProfile.VPAs {
			if vpa.State == api.DeploymentStateDisposed.String() {
				disposeCount++
			}
		}
		if disposeCount == 4 {
			break
		}
	}
	require.Equal(t, 4, disposeCount, "Expected 4 deployments to be disposed")
}
