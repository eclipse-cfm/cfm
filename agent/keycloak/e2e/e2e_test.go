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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/agent/keycloak/launcher"
	"github.com/eclipse-cfm/cfm/assembly/vault"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"
)

const (
	streamName = "cfm-stream"
	cfmBucket  = "cfm-bucket"

	orchestrationID = "1234"
)

func Test_SuccessfulDeployment(t *testing.T) {
	ctx := context.Background()

	err, natsContainer, vaultContainerResult, vaultSetupResult := setupEnvironment(t, ctx)

	shutdownChannel := make(chan struct{})
	go func() {
		launcher.LaunchAndWaitSignal(shutdownChannel)
	}()

	err = createOrchestration(ctx, orchestrationID, natsContainer.Client)
	require.NoError(t, err)

	err = publishActivityMessage(ctx, orchestrationID, api.DeployDiscriminator, natsContainer.Client)
	require.NoError(t, err)

	vaultClient, err := vault.NewVaultClient(vaultContainerResult.URL, vaultSetupResult.ClientID, vaultSetupResult.ClientSecret, vaultSetupResult.TokenURL)
	require.NoError(t, err)
	defer vaultClient.Close()

	var apiAccessClientID, vaultAccessClientId string
	require.Eventually(t, func() bool {
		oEntry, err := natsContainer.Client.KVStore.Get(ctx, "1234")
		if err != nil {
			return false
		}
		var orchestration api.Orchestration
		err = json.Unmarshal(oEntry.Value(), &orchestration)
		if err != nil {
			return false
		}
		if orchestration.State == api.OrchestrationStateCompleted {
			apiAccessClientID = orchestration.ProcessingData["clientID.apiAccess"].(string)
			vaultAccessClientId = orchestration.ProcessingData["clientID.vaultAccess"].(string)
			return true
		}
		return false
	}, 10*time.Second, 10*time.Millisecond, "Orchestration did not complete in time")

	require.NotEmpty(t, apiAccessClientID, "Expected clientID.apiAccess to be set")
	require.NotEmpty(t, vaultAccessClientId, "Expected clientID.vaultAccess to be set")
	apiAccessSecret, err := vaultClient.ResolveSecret(ctx, apiAccessClientID)
	require.NoError(t, err, "Failed to resolve secret")
	require.NotEmpty(t, apiAccessSecret, "Expected api access client secret to be set")
	vaultAccessSecret, err := vaultClient.ResolveSecret(ctx, vaultAccessClientId)
	require.NoError(t, err, "Failed to resolve secret")
	require.NotEmpty(t, vaultAccessSecret, "Expected vault access client secret to be set")

	shutdownChannel <- struct{}{}
}

func Test_UnsupportedDiscriminator(t *testing.T) {
	ctx := context.Background()

	err, natsContainer, _, _ := setupEnvironment(t, ctx)

	shutdownChannel := make(chan struct{})
	go func() {
		launcher.LaunchAndWaitSignal(shutdownChannel)
	}()

	err = createOrchestration(ctx, orchestrationID, natsContainer.Client)
	require.NoError(t, err)

	err = publishActivityMessage(ctx, orchestrationID, "foobar", natsContainer.Client)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		oEntry, err := natsContainer.Client.KVStore.Get(ctx, "1234")
		if err != nil {
			return false
		}
		var orchestration api.Orchestration
		err = json.Unmarshal(oEntry.Value(), &orchestration)
		if err != nil {
			return false
		}
		if orchestration.State == api.OrchestrationStateErrored {
			require.Nil(t, orchestration.ProcessingData["clientID.apiAccess"])
			require.Nil(t, orchestration.ProcessingData["clientID.vaultAccess"])
			return true
		}
		return false
	}, 10*time.Second, 10*time.Millisecond, "Orchestration did not complete in time")

	shutdownChannel <- struct{}{}
}

func setupEnvironment(t *testing.T, ctx context.Context) (error, *natsfixtures.NatsTestContainer, *vault.ContainerResult, *vault.SetupResult) {
	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("failed to create network: %s", err)
	}

	keycloakContainerResult, err := vault.StartKeycloakContainer(ctx, net.Name)
	require.NoError(t, err, "Failed to start Keycloak container")

	nt, err := natsfixtures.SetupNatsContainer(ctx, cfmBucket)

	require.NoError(t, err)
	vaultContainerResult, err := vault.StartVaultContainer(ctx, net.Name)
	require.NoError(t, err, "Failed to start Vault container")
	kcHost := fmt.Sprintf("http://%s:%d", keycloakContainerResult.ContainerName, 8080)
	setupResult, err := vault.SetupVault(vaultContainerResult.URL, vaultContainerResult.Token, keycloakContainerResult.URL, kcHost)
	if err != nil {
		vaultContainerResult.Cleanup()
		t.Fatalf("Failed to setup Vault: %v", err)
	}

	_ = os.Setenv("KCAGENT_VAULT_URL", vaultContainerResult.URL)
	_ = os.Setenv("KCAGENT_VAULT_CLIENTID", setupResult.ClientID)
	_ = os.Setenv("KCAGENT_VAULT_CLIENTSECRET", setupResult.ClientSecret)
	_ = os.Setenv("KCAGENT_VAULT_TOKENURL", setupResult.TokenURL)
	_ = os.Setenv("KCAGENT_VAULT_PATH", setupResult.VaultPath)

	_ = os.Setenv("KCAGENT_URI", nt.URI)
	_ = os.Setenv("KCAGENT_BUCKET", cfmBucket)
	_ = os.Setenv("KCAGENT_STREAM", streamName)
	_ = os.Setenv("KCAGENT_KEYCLOAK_URL", keycloakContainerResult.URL)
	_ = os.Setenv("KCAGENT_KEYCLOAK_CLIENTID", "admin-cli")
	_ = os.Setenv("KCAGENT_KEYCLOAK_USERNAME", "admin")
	_ = os.Setenv("KCAGENT_KEYCLOAK_PASSWORD", "admin")
	_ = os.Setenv("KCAGENT_KEYCLOAK_REALM", "master")
	return err, nt, vaultContainerResult, setupResult
}

func createOrchestration(ctx context.Context, id string, client *natsclient.NatsClient) error {
	orchestration := api.Orchestration{
		ID:                id,
		CorrelationID:     "correlation-id",
		State:             0,
		OrchestrationType: "test",
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "test-activity", Type: launcher.ActivityType},
				},
			},
		},
		ProcessingData: make(map[string]any),
		OutputData:     make(map[string]any),
		Completed:      make(map[string]struct{}),
	}
	serialized, err := json.Marshal(orchestration)
	if err != nil {
		return err
	}
	_, err = client.KVStore.Update(ctx, "1234", serialized, 0)
	return err
}

func publishActivityMessage(ctx context.Context, id string, d api.Discriminator, client *natsclient.NatsClient) error {
	message := &api.ActivityMessage{
		OrchestrationID: id,
		Activity: api.Activity{
			ID:            "test-activity",
			Type:          launcher.ActivityType,
			Discriminator: d,
			Inputs:        make([]api.MappingEntry, 0),
			DependsOn:     make([]string, 0),
		},
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	subject := natsclient.CFMSubjectPrefix + "." + launcher.ActivityType
	_, err = client.JetStream.Publish(ctx, subject, data)
	return nil
}
