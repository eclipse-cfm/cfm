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

	"github.com/metaform/connector-fabric-manager/common/natsfixtures"
	"github.com/metaform/connector-fabric-manager/common/sqlstore"
	. "github.com/metaform/connector-fabric-manager/common/stream"
	"github.com/metaform/connector-fabric-manager/e2e/e2efixtures"
	"github.com/metaform/connector-fabric-manager/pmanager/model/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_VerifyDefinitionOperations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, cfmBucket)
	require.NoError(t, err)

	pg, dsn, err := sqlstore.SetupTestContainer(t)
	require.NoError(t, err)
	defer pg.Terminate(context.Background())

	defer natsfixtures.TeardownNatsContainer(ctx, nt)
	defer cleanup()

	client := launchPlatform(t, nt.URI, dsn)

	waitTManager(t, client)
	waitPManager(t, client)

	var activityDefinitions []v1alpha1.ActivityDefinitionDto

	err = e2efixtures.CreateTestActivityDefinition(client)
	require.NoError(t, err)

	err = client.GetPManager("activity-definitions", &activityDefinitions)
	require.NoError(t, err)
	assert.Equal(t, 1, len(activityDefinitions))

	err = e2efixtures.CreateTestOrchestrationDefinitions(client)
	require.NoError(t, err)

	var orchestrationDefinitions []v1alpha1.OrchestrationDefinitionDto
	err = client.GetPManager("orchestration-definitions", &orchestrationDefinitions)
	require.NoError(t, err)
	assert.Equal(t, 2, len(orchestrationDefinitions))

	for _, definition := range activityDefinitions {
		// Verify delete returns error when definition is referenced by an orchestration definition
		err = client.DeleteToPManager(fmt.Sprintf("activity-definitions/%s", definition.Type))
		require.Error(t, err)
		require.Contains(t, err.Error(), "referenced by an orchestration definition")
	}

	keys := Map(
		From(orchestrationDefinitions), func(o v1alpha1.OrchestrationDefinitionDto) string {
			return o.TemplateRef
		}).
		Distinct()
	for key := range keys.Seq() {
		err = client.DeleteToPManager(fmt.Sprintf("orchestration-definitions/%s", key))
		require.NoError(t, err)
	}

	orchestrationDefinitions = nil
	err = client.GetPManager("orchestration-definitions", &orchestrationDefinitions)
	require.NoError(t, err)
	assert.Empty(t, len(orchestrationDefinitions))

	// Cleanup previously created activity definitions which must be done after orchestration definitions are removed
	//to avoid reference errors
	for _, definition := range activityDefinitions {
		err = client.DeleteToPManager(fmt.Sprintf("activity-definitions/%s", definition.Type))
		require.NoError(t, err)
	}
	err = client.GetPManager("activity-definitions", &activityDefinitions)
	require.NoError(t, err)
	assert.Empty(t, len(activityDefinitions))
}
