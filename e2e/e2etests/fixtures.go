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

	"github.com/eclipse-cfm/cfm/common/fixtures"
	"github.com/eclipse-cfm/cfm/e2e/e2efixtures"
	testLauncher "github.com/eclipse-cfm/cfm/e2e/testagent/launcher"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	plauncher "github.com/eclipse-cfm/cfm/pmanager/cmd/server/launcher"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout = 30 * time.Second
	streamName  = "cfm-stream"
	cfmBucket   = "cfm-bucket"
)

// launchPlatformWithAgent launches the Provision Manager and the test agent.
func launchPlatformWithAgent(t *testing.T, natsURI string, pgDsn string) *e2efixtures.ApiClient {
	shutdownChannel, client := launchPlatform(t, natsURI, pgDsn)
	go func() {
		testLauncher.Launch(shutdownChannel)
	}()

	return client
}

func launchPlatform(t *testing.T, natsURI string, pgDsn string) (chan struct{}, *e2efixtures.ApiClient) {
	fixtures.IsolateConfig(t)

	_ = os.Setenv("PM_POSTGRES", "true")
	_ = os.Setenv("PM_DSN", pgDsn)
	_ = os.Setenv("PM_URI", natsURI)
	_ = os.Setenv("PM_NATS_AUTH_ALLOWANONYMOUS", "true")
	_ = os.Setenv("PM_BUCKET", cfmBucket)
	_ = os.Setenv("PM_STREAM", streamName)
	_ = os.Setenv("PM_AUTH_ENABLED", "false")
	_ = os.Setenv("PM_AUTH_ALLOWDISABLED", "true")

	_ = os.Setenv("TESTAGENT_URI", natsURI)
	_ = os.Setenv("TESTAGENT_NATS_AUTH_ALLOWANONYMOUS", "true")
	_ = os.Setenv("TESTAGENT_BUCKET", cfmBucket)
	_ = os.Setenv("TESTAGENT_STREAM", streamName)

	pPort := fixtures.GetRandomPort(t)
	_ = os.Setenv("PM_HTTPPORT", strconv.Itoa(pPort))

	shutdownChannel := make(chan struct{})
	go func() {
		plauncher.Launch(shutdownChannel)
	}()

	return shutdownChannel, e2efixtures.NewApiClient(fmt.Sprintf("http://localhost:%d/api/v1alpha1", pPort))
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
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, err)
}

func cleanup() {
	_ = os.Unsetenv("PM_URI")
	_ = os.Unsetenv("PM_NATS_AUTH_ALLOWANONYMOUS")
	_ = os.Unsetenv("PM_AUTH_ALLOWDISABLED")
	_ = os.Unsetenv("PM_BUCKET")
	_ = os.Unsetenv("PM_STREAM")

	_ = os.Unsetenv("TESTAGENT_URI")
	_ = os.Unsetenv("TESTAGENT_NATS_AUTH_ALLOWANONYMOUS")
	_ = os.Unsetenv("TESTAGENT_BUCKET")
	_ = os.Unsetenv("TESTAGENT_STREAM")

	_ = os.Unsetenv("PM_HTTPPORT")
}
