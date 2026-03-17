package vault

import (
	"context"
	"fmt"
	"testing"

	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"
)

func TestVaultServiceAssembly_Init(t *testing.T) {
	ctx := context.Background()
	result := setupTest(ctx, t)
	defer result.cleanup()

	assembly := &VaultServiceAssembly{}

	vConfig := viper.New()
	vConfig.Set(urlKey, result.url)
	vConfig.Set(clientIDKey, result.clientID)
	vConfig.Set(clientSecretKey, result.clientSecret)
	vConfig.Set(tokenURLKey, result.tokenURL)
	vConfig.Set(vaultPathKey, vaultPath)

	ictx := &system.InitContext{
		StartContext: system.StartContext{
			Registry:   system.NewServiceRegistry(),
			LogMonitor: system.NoopMonitor{},
			Config:     vConfig,
			Mode:       system.DebugMode,
		},
	}
	err := assembly.Init(ictx)
	require.NoError(t, err)

	client := ictx.Registry.Resolve(serviceapi.VaultKey).(serviceapi.VaultClient)
	require.NotNil(t, client)

	err = client.StoreSecret(ctx, "test-secret", "test-value")
	require.NoError(t, err)

	val, err := client.ResolveSecret(ctx, "test-secret")
	require.NoError(t, err)
	require.Equal(t, "test-value", val, "Expected secret value to match")
}

type TestSetupResult struct {
	url          string
	clientID     string
	clientSecret string
	tokenURL     string
	cleanup      func()
}

func setupTest(ctx context.Context, t *testing.T) TestSetupResult {
	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("failed to create network: %s", err)
	}

	// starting keycloak is necessary - Vault won't let us configure JWT if the JWKS endpoint is not reachable
	keycloakContainerResult, err := StartKeycloakContainer(ctx, net.Name)
	require.NoError(t, err, "Failed to start Keycloak container")

	containerResult, err := StartVaultContainer(ctx, net.Name)
	require.NoError(t, err, "Failed to start Vault container")

	kcHost := fmt.Sprintf("http://%s:%d", keycloakContainerResult.ContainerName, 8080)
	setupResult, err := SetupVault(containerResult.URL, containerResult.Token, keycloakContainerResult.URL, kcHost)
	if err != nil {
		containerResult.Cleanup()
		t.Fatalf("Failed to setup Vault: %v", err)
	}

	return TestSetupResult{
		url:          containerResult.URL,
		clientID:     setupResult.ClientID,
		clientSecret: setupResult.ClientSecret,
		tokenURL:     setupResult.TokenURL,
		cleanup:      containerResult.Cleanup,
	}
}
