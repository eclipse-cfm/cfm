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
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/metaform/connector-fabric-manager/common/fixtures"
	"github.com/metaform/connector-fabric-manager/e2e/e2efixtures"
	testLauncher "github.com/metaform/connector-fabric-manager/e2e/testagent/launcher"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	plauncher "github.com/metaform/connector-fabric-manager/pmanager/cmd/server/launcher"
	tlauncher "github.com/metaform/connector-fabric-manager/tmanager/cmd/server/launcher"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout = 30 * time.Second
	streamName  = "cfm-stream"
	cfmBucket   = "cfm-bucket"
)

func launchPlatformWithAgent(t *testing.T, natsURI string, pgDsn string) *e2efixtures.ApiClient {
	shutdownChannel, client := launchPlatform(t, natsURI, pgDsn)
	go func() {
		testLauncher.Launch(shutdownChannel)
	}()

	return client
}

func launchPlatform(t *testing.T, natsURI string, pgDsn string) (chan struct{}, *e2efixtures.ApiClient) {
	_ = os.Setenv("TM_POSTGRES", "true")
	_ = os.Setenv("TM_DSN", pgDsn)
	_ = os.Setenv("TM_URI", natsURI)
	_ = os.Setenv("TM_BUCKET", cfmBucket)
	_ = os.Setenv("TM_STREAM", streamName)

	_ = os.Setenv("PM_POSTGRES", "true")
	_ = os.Setenv("PM_DSN", pgDsn)
	_ = os.Setenv("PM_URI", natsURI)
	_ = os.Setenv("PM_BUCKET", cfmBucket)
	_ = os.Setenv("PM_STREAM", streamName)

	_ = os.Setenv("TESTAGENT_URI", natsURI)
	_ = os.Setenv("TESTAGENT_BUCKET", cfmBucket)
	_ = os.Setenv("TESTAGENT_STREAM", streamName)

	tPort := fixtures.GetRandomPort(t)
	_ = os.Setenv("TM_HTTPPORT", strconv.Itoa(tPort))
	pPort := fixtures.GetRandomPort(t)
	_ = os.Setenv("PM_HTTPPORT", strconv.Itoa(pPort))

	shutdownChannel := make(chan struct{})
	go func() {
		plauncher.Launch(shutdownChannel)
	}()

	go func() {
		tlauncher.Launch(shutdownChannel)
	}()

	return shutdownChannel, e2efixtures.NewApiClient(fmt.Sprintf("http://localhost:%d/api/v1alpha1", tPort), fmt.Sprintf("http://localhost:%d/api/v1alpha1", pPort))
}

func waitTManager(t *testing.T, client *e2efixtures.ApiClient) {
	var err error
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		// Wait until a tenant can be created
		if tenant, err := e2efixtures.CreateTenant(client, map[string]any{"group": "suppliers"}); err == nil {
			err = client.DeleteToTManager(fmt.Sprintf("tenants/%s", tenant.ID))
			require.NoError(t, err)
			break
		}
	}
	require.NoError(t, err)
}

func waitPManager(t *testing.T, client *e2efixtures.ApiClient) {
	var err error
	for start := time.Now(); time.Since(start) < 5*time.Second; {
		// Wait until an activity definition can be created
		requestBody := api.ActivityDefinition{
			Type: "boot-activity",
		}
		if err = client.PostToPManager("activity-definitions", requestBody); err == nil {
			err = client.DeleteToPManager("activity-definitions/boot-activity")
			require.NoError(t, err)
			break
		}
	}
	require.NoError(t, err)
}

func cleanup() {
	_ = os.Unsetenv("TM_URI")
	_ = os.Unsetenv("TM_BUCKET")
	_ = os.Unsetenv("TM_STREAM")

	_ = os.Unsetenv("PM_URI")
	_ = os.Unsetenv("PM_BUCKET")
	_ = os.Unsetenv("PM_STREAM")

	_ = os.Unsetenv("TESTAGENT_URI")
	_ = os.Unsetenv("TESTAGENT_BUCKET")
	_ = os.Unsetenv("TESTAGENT_STREAM")

	_ = os.Unsetenv("TM_HTTPPORT")
	_ = os.Unsetenv("PM_HTTPPORT")
}
