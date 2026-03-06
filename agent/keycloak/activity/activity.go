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

package activity

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/google/uuid"
)

const (
	jsonContentType         = "application/json"
	contentTypeHeader       = "Content-Type"
	authHeader              = "Authorization"
	clientUrl               = "%s/admin/realms/%s/clients"
	vaultAccessClientIDKey  = "clientID.vaultAccess"
	apiAccessClientIDKey    = "clientID.apiAccess"
	participantContextIDKey = "participantContextId"
)

type Config struct {
	KeycloakURL string
	Realm       string
	Monitor     system.LogMonitor
	VaultClient serviceapi.VaultClient
	HTTPClient  *http.Client
	ClientId    string
	Username    string
	Password    string
}

// KeyCloakActivityProcessor creates a confidential client in Keycloak and stores the client secret in Vault for use by
// other processors. The client ID is returned as a value in the context.
type KeyCloakActivityProcessor struct {
	keycloakURL string
	clientId    string
	username    string
	password    string
	realm       string
	monitor     system.LogMonitor
	httpClient  *http.Client
	vaultClient serviceapi.VaultClient
}

type KeycloakClientData struct {
	ClientId                  string           `json:"clientId"`
	Name                      string           `json:"name"`
	Description               string           `json:"description"`
	Enabled                   bool             `json:"enabled"`
	Secret                    string           `json:"secret"`
	Protocol                  string           `json:"protocol"`
	PublicClient              bool             `json:"publicClient"`
	ServiceAccountsEnabled    bool             `json:"serviceAccountsEnabled"`
	StandardFlowEnabled       bool             `json:"standardFlowEnabled"`
	DirectAccessGrantsEnabled bool             `json:"directAccessGrantsEnabled"`
	FullScopeAllowed          bool             `json:"fullScopeAllowed"`
	ProtocolMappers           []map[string]any `json:"protocolMappers"`
}

type KeycloakClientDataOption func(*KeycloakClientData)

func WithName(name string) KeycloakClientDataOption {
	return func(c *KeycloakClientData) {
		c.Name = name
	}
}

func WithDescription(desc string) KeycloakClientDataOption {
	return func(c *KeycloakClientData) {
		c.Description = desc
	}
}

func WithEnabled(enabled bool) KeycloakClientDataOption {
	return func(c *KeycloakClientData) {
		c.Enabled = enabled
	}
}

func WithPublicClient(public bool) KeycloakClientDataOption {
	return func(c *KeycloakClientData) {
		c.PublicClient = public
	}
}

func WithProtocolMappers(mappers []map[string]any) KeycloakClientDataOption {
	return func(c *KeycloakClientData) {
		c.ProtocolMappers = mappers
	}
}

func WithClientID(clientID string) KeycloakClientDataOption {
	return func(c *KeycloakClientData) {
		c.ClientId = clientID
	}
}

func WithClientSecret(secret string) KeycloakClientDataOption {
	return func(c *KeycloakClientData) {
		c.Secret = secret
	}
}

func newKeycloakClientData(participantContextId string, opts ...KeycloakClientDataOption) (*KeycloakClientData, error) {
	clientID := generateClientID()
	clientSecret, err := generateClientSecret()
	if err != nil {
		return nil, err
	}
	clientData := KeycloakClientData{
		ClientId:                  clientID,
		Secret:                    clientSecret,
		Name:                      clientID + " Client",
		Enabled:                   true,
		Protocol:                  "openid-connect",
		PublicClient:              false,
		ServiceAccountsEnabled:    true,
		StandardFlowEnabled:       false,
		DirectAccessGrantsEnabled: false,
		FullScopeAllowed:          true,
	}

	for _, opt := range opts {
		opt(&clientData)
	}

	clientData.ProtocolMappers = []map[string]any{
		{
			"name":            "participantContextId",
			"protocol":        "openid-connect",
			"protocolMapper":  "oidc-hardcoded-claim-mapper",
			"consentRequired": false,
			"config": map[string]string{
				"claim.name":           "participant_context_id",
				"claim.value":          participantContextId,
				"jsonType.label":       "String",
				"access.token.claim":   "true",
				"id.token.claim":       "true",
				"userinfo.token.claim": "true",
			},
		},
		{
			"name":            "role",
			"protocol":        "openid-connect",
			"protocolMapper":  "oidc-hardcoded-claim-mapper",
			"consentRequired": false,
			"config": map[string]string{
				"claim.name":           "role",
				"claim.value":          "participant",
				"jsonType.label":       "String",
				"access.token.claim":   "true",
				"id.token.claim":       "true",
				"userinfo.token.claim": "true",
			},
		},
	}

	return &clientData, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewProcessor creates a new KeyCloakActivityProcessor instance
func NewProcessor(config *Config) *KeyCloakActivityProcessor {
	return &KeyCloakActivityProcessor{
		keycloakURL: config.KeycloakURL,
		clientId:    config.ClientId,
		username:    config.Username,
		password:    config.Password,
		realm:       config.Realm,
		monitor:     config.Monitor,
		httpClient:  config.HTTPClient,
		vaultClient: config.VaultClient,
	}
}

func (p KeyCloakActivityProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if ctx.Discriminator() == api.DeployDiscriminator {
		return p.handleDeployAction(ctx)
	}

	if ctx.Discriminator() == api.DisposeDiscriminator {
		return p.handleDisposeAction(ctx)
	}
	return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("the '%s' discriminator is not supported", ctx.Discriminator())}
}

// handleDeployAction handles the deployment process, creating required Keycloak clients for API and Vault access.
func (p KeyCloakActivityProcessor) handleDeployAction(ctx api.ActivityContext) api.ActivityResult {
	clientIDSlug := generateClientID()

	// create Keycloak client for API access
	participantContextID := clientIDSlug

	apiClient, err := newKeycloakClientData(participantContextID, WithClientID(participantContextID), WithName("API Access Client"), WithDescription("Client for accessing the VPA's Administration APIs"), WithEnabled(true))
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	apiClientResult := p.provisionConfidentialClient(apiClient, ctx)
	p.monitor.Debugf("created API Access client: %s", apiClient.ClientId)
	ctx.SetValue(apiAccessClientIDKey, apiClient.ClientId)
	ctx.SetOutputValue(participantContextIDKey, participantContextID)
	if apiClientResult.Result != api.ActivityResultComplete {
		p.monitor.Warnw("Provisioning API Access client not complete. Result was %s, error: %s", apiClientResult.Result, apiClientResult.Error)
		return apiClientResult
	}

	// create a Vault access client in Keycloak
	vaultAccessClientID := clientIDSlug + "-vault"
	vaultAccessClient, err := newKeycloakClientData(participantContextID, WithClientID(vaultAccessClientID), WithName("Vault Access Client"), WithDescription("Client for Vault to access Keycloak"), WithEnabled(true))
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	vaultClientResult := p.provisionConfidentialClient(vaultAccessClient, ctx)
	p.monitor.Debugf("created Vault Access client: %s", vaultAccessClient.ClientId)
	ctx.SetValue(vaultAccessClientIDKey, vaultAccessClient.ClientId)

	if vaultClientResult.Result != api.ActivityResultComplete {
		p.monitor.Warnw("Provisioning Vault Access client not complete. Result was %s, error: %s", vaultClientResult.Result, vaultClientResult.Error)
	}
	return vaultClientResult
}

// handleDisposeAction handles the disposal of Keycloak clients, removing API and Vault access clients if they exist.
func (p KeyCloakActivityProcessor) handleDisposeAction(ctx api.ActivityContext) api.ActivityResult {
	apiAccessID := ctx.Values()[apiAccessClientIDKey].(string)
	vaultAccessID := ctx.Values()[vaultAccessClientIDKey].(string)
	if vaultAccessID != "" && apiAccessID != "" {
		vaultErr := p.deleteClient(vaultAccessID)
		p.monitor.Debugf("deleted Vault Access client: %s", vaultAccessID)
		apiErr := p.deleteClient(apiAccessID)
		p.monitor.Debugf("deleted API Access client: %s", apiAccessID)

		var errors []error
		if vaultErr != nil {
			errors = append(errors, vaultErr)
		}
		if apiErr != nil {
			errors = append(errors, apiErr)
		}
		if len(errors) > 0 {
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("could not roll back Keycloak clients: %v", errors)}
		}
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}

	if apiAccessID == "" {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("could not roll back Keycloak API access client: the '%s' output value is not set", apiAccessClientIDKey)}
	}
	// implicitly, vaultAccessID is empty
	return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("could not roll back Keycloak vault access client: the '%s' output value is not set", vaultAccessClientIDKey)}

}

// provisionConfidentialClient creates a confidential client in Keycloak and stores the client secret in Vault for use by
// other processors. The client ID is returned as a value in the context.
// TODO support idempotent provisioning
func (p KeyCloakActivityProcessor) provisionConfidentialClient(client *KeycloakClientData, ctx api.ActivityContext) api.ActivityResult {
	err := p.createClient(client)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	err = p.vaultClient.StoreSecret(ctx.Context(), client.ClientId, client.Secret)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// deleteClient deletes a client in Keycloak. Important: pass the client ID, _not_ the internal UUID!
func (p KeyCloakActivityProcessor) deleteClient(clientID string) error {

	// the human-readable client-ID cannot be used to delete the client directly, we need to look up KC's internal UUID
	clientUUID, err := p.getClientUUID(clientID)
	// clientURL should be <HOST>/admin/realms/edcv/clients/<CLIENT_UID>
	clientURL := fmt.Sprintf(clientUrl, p.keycloakURL, p.realm)
	clientURL = fmt.Sprintf("%s/%s", clientURL, clientUUID)

	req, err := http.NewRequest(http.MethodDelete, clientURL, nil)
	if err != nil {
		return fmt.Errorf("error creating client request: %w", err)
	}

	token, err := p.getToken()
	if err != nil {
		return fmt.Errorf("error authenticating with Keycloak: %w", err)
	}
	req.Header.Set(authHeader, fmt.Sprintf("Bearer %s", token))
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete client request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create client operation failed: status %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// createClient creates a confidential client with the specified secret
func (p KeyCloakActivityProcessor) createClient(clientData *KeycloakClientData) error {
	clientURL := fmt.Sprintf(clientUrl, p.keycloakURL, p.realm)

	jsonData, err := json.Marshal(clientData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, clientURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating client request: %w", err)
	}

	req.Header.Set(contentTypeHeader, jsonContentType)
	token, err := p.getToken()
	if err != nil {
		return fmt.Errorf("error authenticating with Keycloak: %w", err)
	}
	req.Header.Set(authHeader, fmt.Sprintf("Bearer %s", token))
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create client request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create client operation failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (p KeyCloakActivityProcessor) getToken() (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", p.keycloakURL)

	formData := fmt.Sprintf("username=%s&password=%s&client_id=%s&grant_type=password",
		p.username, p.password, p.clientId)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("error creating token request: %w", err)
	}

	req.Header.Set(contentTypeHeader, "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("error decoding token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func (p KeyCloakActivityProcessor) getClientUUID(clientID string) (string, error) {
	clientURL := fmt.Sprintf(clientUrl, p.keycloakURL, p.realm)
	clientURL = fmt.Sprintf("%s?clientId=%s", clientURL, clientID)

	token, err := p.getToken()
	if err != nil {
		return "", fmt.Errorf("error authenticating with Keycloak: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, clientURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating client request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get client request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get client operation failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	var clientResp []map[string]any
	if err := json.Unmarshal(body, &clientResp); err != nil {
		return "", fmt.Errorf("error decoding client response: %w", err)
	}
	if len(clientResp) != 1 {
		return "", fmt.Errorf("expected to find 1 client for client-id '%s', found %d", clientID, len(clientResp))
	}
	return clientResp[0]["id"].(string), nil
}

// generateClientSecret generates a random secret using encoding.
func generateClientSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateClientID generates a unique client ID that complies with Keycloak and typical Vault requirements
func generateClientID() string {

	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
