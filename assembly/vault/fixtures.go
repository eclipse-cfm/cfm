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

package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/metaform/connector-fabric-manager/assembly/serviceapi"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	networkName          = "vault-test-network"
	vaultImage           = "hashicorp/vault:latest"
	vaultPort            = "8200"
	vaultRootToken       = "myroot"
	vaultPath            = "testpath"
	vaultRequestTimeout  = 30 * time.Second
	containerStartupTime = 15 * time.Second

	keycloakImage         = "keycloak/keycloak:latest"
	keycloakAdminUser     = "admin"
	keycloakAdminPassword = "admin"
	keycloakPort          = "8080"
)

func NewVaultClient(vaultURL, oauth2ClientID, oauth2ClientSecret string, tokenUrl string) (serviceapi.VaultClient, error) {
	return createVaultClient(vaultURL, oauth2ClientID, oauth2ClientSecret, tokenUrl)
}

type ContainerResult struct {
	URL           string
	Token         string
	Cleanup       func()
	ContainerName string
}

// StartVaultContainer starts a Vault container and returns a ContainerResult containing info about the container, and clean-up function
func StartVaultContainer(ctx context.Context, networkName string) (*ContainerResult, error) {
	name := uuid.New().String()
	req := testcontainers.ContainerRequest{
		Image:        vaultImage,
		ExposedPorts: []string{vaultPort},
		Networks:     []string{networkName},
		Name:         name,
		Env: map[string]string{
			"VAULT_DEV_ROOT_TOKEN_ID": vaultRootToken,
		},
		Cmd: []string{"server", "-dev"},
		WaitingFor: wait.ForLog("WARNING! dev mode is enabled!").
			WithStartupTimeout(containerStartupTime),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, vaultPort)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	vaultURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	cleanup := func() {
		_ = container.Terminate(context.Background())
	}

	return &ContainerResult{
		URL:           vaultURL,
		ContainerName: name,
		Token:         vaultRootToken,
		Cleanup:       cleanup,
	}, nil
}

// StartKeycloakContainer starts a Keycloak container and returns a ContainerResult containing info about the container, and clean-up function. This does not include a "token" field
func StartKeycloakContainer(ctx context.Context, networkName string) (*ContainerResult, error) {
	name := uuid.New().String()
	req := testcontainers.ContainerRequest{
		Image:        keycloakImage,
		ExposedPorts: []string{keycloakPort},
		Networks:     []string{networkName},
		Name:         name,
		Env: map[string]string{
			"KEYCLOAK_ADMIN":              keycloakAdminUser,
			"KC_BOOTSTRAP_ADMIN_USERNAME": keycloakAdminUser,
			"KC_BOOTSTRAP_ADMIN_PASSWORD": keycloakAdminPassword,
			"KC_HEALTH_ENABLED":           "true",
		},
		Cmd:        []string{"start-dev", "--health-enabled=true"},
		WaitingFor: wait.ForLog("Profile dev activated"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault container: %w", err)
	}

	// this is the only reliable way to disable SSL enforcement. not env-var or combinations thereof seem to work.
	_, _, err = container.Exec(ctx, []string{
		"/opt/keycloak/bin/kcadm.sh",
		"update", "realms/master",
		"-s", "sslRequired=NONE",
		"--server", "http://localhost:" + keycloakPort,
		"--realm", "master",
		"--user", keycloakAdminUser,
		"--password", keycloakAdminPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to disable SSL enforcement in Keycloak: %w", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, keycloakPort)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	kcUrl := fmt.Sprintf("http://%s:%s", host, port.Port())

	cleanup := func() {
		_ = container.Terminate(context.Background())
	}
	return &ContainerResult{
		URL:           kcUrl,
		Token:         "",
		ContainerName: name,
		Cleanup:       cleanup,
	}, nil
}

type SetupResult struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	VaultPath    string
}

// SetupVault sets up Vault with JWT authentication using Keycloak as the identity provider. It creates a Keycloak client and configures Vault accordingly.
// vaultURL is the URL of the Vault server.
// rootToken is the root token for Vault.
// keycloakURL is the URL of the Keycloak server.
// keycloakHostInternal is the internal hostname of the Keycloak server accessible from the Vault container. In a docker environment, this is the container name plus the container (internal) port
func SetupVault(vaultURL string, rootToken string, keycloakURL string, keycloakHostInternal string) (*SetupResult, error) {

	clientId, clientSecret, err := createKeycloakUser(keycloakURL, keycloakAdminUser, keycloakAdminPassword)

	if err != nil {
		return nil, err
	}

	if err := setupVaultJwtAuth(vaultURL, rootToken, keycloakHostInternal); err != nil {
		return nil, fmt.Errorf("failed to setup Vault JWT auth: %w", err)
	}

	return &SetupResult{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		TokenURL:     keycloakURL + "/realms/master/protocol/openid-connect/token",
		VaultPath:    vaultPath,
	}, nil
}

func setupTestFixtures(ctx context.Context, t *testing.T) (*vaultClient, func()) {

	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("failed to create network: %s", err)
	}

	vaultContainerResult, err := StartVaultContainer(ctx, net.Name)
	require.NoError(t, err, "Failed to start Vault container")

	keycloakContainerResult, err := StartKeycloakContainer(ctx, net.Name)
	require.NoError(t, err, "Failed to start Keycloak container")

	clientId, clientSecret, err := createKeycloakUser(keycloakContainerResult.URL, keycloakAdminUser, keycloakAdminPassword)
	require.NoError(t, err, "Failed to create Keycloak user")

	keycloakHostInternal := fmt.Sprintf("http://%s:%s", keycloakContainerResult.ContainerName, keycloakPort)
	if err := setupVaultJwtAuth(vaultContainerResult.URL, vaultContainerResult.Token, keycloakHostInternal); err != nil {
		t.Fatalf("Failed to setup Vault JWT auth: %v", err)
	}

	client, err := createVaultClient(vaultContainerResult.URL, clientId, clientSecret, keycloakContainerResult.URL+"/realms/master/protocol/openid-connect/token")
	if err != nil {
		vaultContainerResult.Cleanup()
		t.Fatalf("Failed to create Vault client: %v", err)
	}

	return client, vaultContainerResult.Cleanup
}

// createKeycloakUser creates a Keycloak client using the specified user and password and returns the client ID and secret. Implicitly
// assumes that the OAuth2 request is using the "password" grant type.
func createKeycloakUser(keycloakBaseUrl string, user string, password string) (string, string, error) {

	httpClient := &http.Client{}
	adminUrl := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", keycloakBaseUrl)
	adminToken, err := getAdminToken(httpClient, adminUrl, user, password, "admin-cli")
	if err != nil {
		return "", "", fmt.Errorf("error creating admin token in Keycloak: %w", err)
	}

	clientURL := keycloakBaseUrl + "/admin/realms/master/clients"
	clientID := "test-client"
	clientSecret := "test-secret"
	clientData := map[string]any{
		"clientId":                  clientID,
		"secret":                    clientSecret,
		"enabled":                   true,
		"protocol":                  "openid-connect",
		"publicClient":              false,
		"serviceAccountsEnabled":    true,
		"standardFlowEnabled":       true,
		"directAccessGrantsEnabled": false,
		"fullScopeAllowed":          true,
		"protocolMappers": []map[string]any{
			{
				"name":            "role",
				"protocol":        "openid-connect",
				"protocolMapper":  "oidc-hardcoded-claim-mapper",
				"consentRequired": false,
				"config": map[string]any{
					"claim.name":           "role",
					"claim.value":          "provisioner",
					"jsonType.label":       "String",
					"access.token.claim":   true,
					"id.token.claim":       true,
					"userinfo.token.claim": true,
				},
			},
		},
	}

	jsonData, err := json.Marshal(clientData)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest(http.MethodPost, clientURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("error creating client request: %w", err)
	}

	req.Header.Set("content-type", "application/json")
	if err != nil {
		return "", "", fmt.Errorf("error authenticating with Keycloak: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", adminToken))
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("create client request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("create client operation failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return clientID, clientSecret, nil
}

// setupVaultJwtAuth configures Vault with JWT authentication using Keycloak as the identity provider.
// vaultUrl is the URL of the Vault server.
// rootToken is the root token for Vault.
// keycloakHostInternal is the internal hostname of the Keycloak server accessible from the Vault container. In a docker environment, this is the container name plus the container (internal) port
func setupVaultJwtAuth(vaultURL string, rootToken string, keycloakHostInternal string) error {
	client := &http.Client{Timeout: vaultRequestTimeout}

	// Enable KV v2 secrets engine
	if err := enableSecretsEngine(client, vaultURL, rootToken, vaultPath, "kv", ""); err != nil {
		return fmt.Errorf("failed to enable KV v2 engine: %w", err)
	}

	// Enable JWT auth method
	if err := enableAuthMethod(client, vaultURL, rootToken, "jwt", "jwt", ""); err != nil {
		return fmt.Errorf("failed to enable JWT auth: %w", err)
	}

	keycloakJwksUrl := fmt.Sprintf("%s/realms/master/protocol/openid-connect/certs", keycloakHostInternal)
	if err := configureJwtAuth(client, vaultURL, rootToken, keycloakJwksUrl, "provisioner"); err != nil {
		return fmt.Errorf("failed to configure JWT auth: %w", err)
	}

	err := createJwtRole(client, vaultURL, keycloakHostInternal, rootToken)
	if err != nil {
		return fmt.Errorf("failed to create JWT Role: %w", err)
	}

	return nil
}

// configureJwtAuth configures the JWT auth method in Vault with the given JWKS URL and default role. The JWKS Url must be accessible from the Vault server.
// In a docker environment, this is typically the container name plus the internal port of the Keycloak server.
func configureJwtAuth(client *http.Client, vaultURL string, rootToken string, keycloakJwksUrl string, defaultRole string) error {
	url := fmt.Sprintf("%s/v1/auth/jwt/config", vaultURL)
	data := map[string]any{
		"default_role": defaultRole,
		"jwks_url":     keycloakJwksUrl,
	}

	_, err := vaultRequest(client, url, rootToken, http.MethodPost, data)
	return err

}

// getAdminToken retrieves an admin token from Keycloak using the provided credentials
func getAdminToken(client *http.Client, adminTokenUrl string, username string, password string, clientId string) (string, error) {

	formData := fmt.Sprintf("username=%s&password=%s&client_id=%s&grant_type=password",
		username, password, clientId)

	req, err := http.NewRequest(http.MethodPost, adminTokenUrl, strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("error creating token request: %w", err)
	}

	req.Header.Set("content-type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	type tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("error decoding token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// enableSecretsEngine enables a secrets engine at a given path
func enableSecretsEngine(client *http.Client, vaultURL, token, path, engineType, description string) error {
	url := fmt.Sprintf("%s/v1/sys/mounts/%s", vaultURL, path)

	data := map[string]any{
		"type":        engineType,
		"description": description,
	}

	_, err := vaultRequest(client, url, token, http.MethodPost, data)
	return err
}

// enableAuthMethod enables an auth method at a given path
func enableAuthMethod(client *http.Client, vaultURL, token, path, methodType, description string) error {
	url := fmt.Sprintf("%s/v1/sys/auth/%s", vaultURL, path)

	data := map[string]any{
		"type":        methodType,
		"description": description,
	}

	_, err := vaultRequest(client, url, token, http.MethodPost, data)
	return err
}

// createJwtRole creates a JWT with a client ID and secret
func createJwtRole(client *http.Client, vaultURL, keycloakHostname string, token string) error {
	// Create a policy that allows reading from kv-v2
	policyURL := fmt.Sprintf("%s/v1/sys/policies/acl/test-policy", vaultURL)
	policyData := map[string]any{
		"policy": fmt.Sprintf(`
			path "%s/data/*" {
			  capabilities = ["create", "read", "update", "delete", "list"]
			}
			path "%s/metadata/*" {
			  capabilities = ["delete", "list", "read"]
			}`, vaultPath, vaultPath),
	}

	if _, err := vaultRequest(client, policyURL, token, http.MethodPut, policyData); err != nil {
		return fmt.Errorf("failed to create policy: %w", err)
	}

	// Create the AppRole role with the policy
	roleURL := fmt.Sprintf("%s/v1/auth/jwt/role/provisioner", vaultURL)
	roleData := map[string]any{
		"role_type":       "jwt",
		"user_claim":      "azp",
		"bound_issuer":    fmt.Sprintf("http://%s:8080/realms/master", keycloakHostname),
		"bound_audiences": []string{"account"},
		"bound_claims": map[string]string{
			"role": "provisioner",
		},
		"clock_skew_leeway": 60,
		"token_policies":    []string{"test-policy"},
		"policies":          []string{"test-policy"},
	}

	if _, err := vaultRequest(client, roleURL, token, http.MethodPost, roleData); err != nil {
		return fmt.Errorf("failed to create JWT role: %w", err)
	}
	return nil

}

// vaultRequest makes an authenticated HTTP request to Vault and returns the response
func vaultRequest(client *http.Client, url, token, method string, data any) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("vault request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func createVaultClient(vaultURL, clientID string, clientSecret string, tokenUrl string) (*vaultClient, error) {
	vaultClient, err := newVaultClient(vaultURL, clientID, clientSecret, tokenUrl, system.NoopMonitor{}, func(client *vaultClient) {
		client.mountPath = vaultPath
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}
	err = vaultClient.init(context.Background())
	if err != nil {
		return nil, err
	}
	return vaultClient, nil
}
