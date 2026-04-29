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
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ConfigOption func(*Config)

func WithIdentityHub(client identityhub.IdentityAPIClient) ConfigOption {
	return func(c *Config) {
		c.IdentityAPIClient = client
	}
}

func validConfig(opts ...ConfigOption) *Config {
	c := Config{
		VaultClient:       NewMockVaultClient("client-123", "secret-123"),
		IdentityAPIClient: MockIdentityHubClient{},
		Client:            &http.Client{},
		LogMonitor:        system.NoopMonitor{},
		TokenURL:          "http://auth.example.com/oauth2/token",
		VaultURL:          "https://vault.example.com:8200",
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

var processingData = map[string]any{
	model.ParticipantIdentifier:         "did:web:participant-abc",
	"clientID.vaultAccess":              "client-123",
	"clientID.apiAccess":                "client-456",
	"cfm.participant.credentialservice": "https://example.com/credentialservice",
	"cfm.participant.protocolservice":   "https://example.com/protocolservice",
}

func TestIHActivityProcessor_ProcessDeploy_WithValidData(t *testing.T) {
	processor := NewProcessor(validConfig())

	activityContext := api.NewActivityContext(context.Background(), "orch-1", api.Activity{
		ID:            "test-activity",
		Type:          "ih",
		Discriminator: api.DeployDiscriminator,
	}, copyOf(processingData), make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)

	// STSClientID must be written back into the context
	stsClientID, ok := activityContext.Value(STSClientIDKey)
	require.True(t, ok)
	assert.Equal(t, "test-sts-clientid", stsClientID)
}

func TestIHActivityProcessor_ProcessDeploy_MissingParticipantID(t *testing.T) {
	processor := NewProcessor(validConfig())
	pd := copyOf(processingData)
	delete(pd, model.ParticipantIdentifier)

	activityContext := api.NewActivityContext(context.Background(), "orch-2", api.Activity{
		ID:   "activity-1",
		Type: "ih",
	}, pd, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	require.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Contains(t, result.Error.Error(), "error processing IH activity")
}

func TestIHActivityProcessor_ProcessDeploy_MissingCredentialServiceURL(t *testing.T) {
	processor := NewProcessor(validConfig())
	pd := copyOf(processingData)
	delete(pd, "cfm.participant.credentialservice")

	activityContext := api.NewActivityContext(context.Background(), "orch-3", api.Activity{
		ID:            "activity-2",
		Type:          "ih",
		Discriminator: api.DeployDiscriminator,
	}, pd, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	require.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Contains(t, result.Error.Error(), "CredentialServiceURL is empty")
}

func TestIHActivityProcessor_ProcessDeploy_MissingProtocolServiceURL(t *testing.T) {
	processor := NewProcessor(validConfig())
	pd := copyOf(processingData)
	delete(pd, "cfm.participant.protocolservice")

	activityContext := api.NewActivityContext(context.Background(), "orch-4", api.Activity{
		ID:            "activity-3",
		Type:          "ih",
		Discriminator: api.DeployDiscriminator,
	}, pd, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	require.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Contains(t, result.Error.Error(), "ProtocolServiceURL is empty")
}

func TestIHActivityProcessor_ProcessDeploy_VaultSecretMissing(t *testing.T) {
	cfg := validConfig()
	cfg.VaultClient = NewMockVaultClient() // empty vault

	processor := NewProcessor(cfg)

	activityContext := api.NewActivityContext(context.Background(), "orch-5", api.Activity{
		ID:            "activity-4",
		Type:          "ih",
		Discriminator: api.DeployDiscriminator,
	}, copyOf(processingData), make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	require.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Contains(t, result.Error.Error(), "error retrieving client secret")
}

func TestIHActivityProcessor_ProcessDeploy_IdentityHubFailure(t *testing.T) {
	processor := NewProcessor(validConfig(WithIdentityHub(MockIdentityHubClient{expectedError: fmt.Errorf("ih unavailable")})))

	activityContext := api.NewActivityContext(context.Background(), "orch-6", api.Activity{
		ID:            "activity-5",
		Type:          "ih",
		Discriminator: api.DeployDiscriminator,
	}, copyOf(processingData), make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	require.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "ih unavailable")
}

func TestIHActivityProcessor_ProcessDispose_Success(t *testing.T) {
	processor := NewProcessor(validConfig())

	activityContext := api.NewActivityContext(context.Background(), "orch-7", api.Activity{
		ID:            "activity-6",
		Type:          "ih",
		Discriminator: api.DisposeDiscriminator,
	}, copyOf(processingData), make(map[string]any))

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

func TestIHActivityProcessor_ProcessDispose_IdentityHubError(t *testing.T) {
	processor := NewProcessor(validConfig(WithIdentityHub(MockIdentityHubClient{expectedError: fmt.Errorf("some error")})))

	activityContext := api.NewActivityContext(context.Background(), "orch-8", api.Activity{
		ID:            "activity-7",
		Type:          "ih",
		Discriminator: api.DisposeDiscriminator,
	}, copyOf(processingData), make(map[string]any))

	result := processor.ProcessDispose(activityContext)

	// errors during dispose are logged as warnings; always returns Complete to unblock subsequent agents
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
}

// --- mocks ---

type MockVaultClient struct {
	cache map[string]string
}

func NewMockVaultClient(secrets ...string) MockVaultClient {
	cache := make(map[string]string)
	for i := 0; i+1 < len(secrets); i += 2 {
		cache[secrets[i]] = secrets[i+1]
	}
	return MockVaultClient{cache: cache}
}

func (m MockVaultClient) ResolveSecret(_ context.Context, path string) (string, error) {
	if v, ok := m.cache[path]; ok {
		return v, nil
	}
	return "", types.ErrNotFound
}

func (m MockVaultClient) StoreSecret(_ context.Context, path string, value string) error {
	m.cache[path] = value
	return nil
}

func (m MockVaultClient) DeleteSecret(_ context.Context, path string) error {
	delete(m.cache, path)
	return nil
}

func (m MockVaultClient) Close() error { return nil }

func (m MockVaultClient) Health(_ context.Context) error { return nil }

type MockIdentityHubClient struct {
	expectedError error
}

func (m MockIdentityHubClient) CreateParticipantContext(_ context.Context, _ identityhub.ParticipantManifest) (*identityhub.CreateParticipantContextResponse, error) {
	if m.expectedError != nil {
		return nil, m.expectedError
	}
	return &identityhub.CreateParticipantContextResponse{
		STSClientID:     "test-sts-clientid",
		STSClientSecret: "test-sts-secret-alias",
	}, nil
}

func (m MockIdentityHubClient) DeleteParticipantContext(_ context.Context, _ string) error {
	return m.expectedError
}

func (m MockIdentityHubClient) RequestCredentials(_ context.Context, _ string, _ identityhub.CredentialRequest) (string, error) {
	panic("not implemented")
}

func (m MockIdentityHubClient) GetCredentialRequestState(_ context.Context, _ string, _ string) (string, error) {
	panic("not implemented")
}

func (m MockIdentityHubClient) QueryCredentialByType(_ context.Context, _ string, _ string) ([]identityhub.VerifiableCredentialResource, error) {
	panic("not implemented")
}

func copyOf(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}
