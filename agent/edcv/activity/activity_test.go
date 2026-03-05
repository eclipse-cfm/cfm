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

	"github.com/metaform/connector-fabric-manager/agent/common/identityhub"
	"github.com/metaform/connector-fabric-manager/agent/edcv/controlplane"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ConfigOptions func(*Config)

func WithIdentityHub(client identityhub.IdentityAPIClient) ConfigOptions {
	return func(config *Config) {
		config.IdentityAPIClient = client
	}
}

func WithControlPlane(client controlplane.ManagementAPIClient) ConfigOptions {
	return func(config *Config) {
		config.ManagementAPIClient = client
	}
}

func validConfig(opts ...ConfigOptions) *Config {
	c := Config{
		VaultClient:         NewMockVaultClient("client-123", "123"),
		ManagementAPIClient: MockManagementApiClient{},
		IdentityAPIClient:   MockIdentityHubClient{},
		Client:              &http.Client{},
		LogMonitor:          system.NoopMonitor{},
		TokenURL:            "http://auth.example.com/oauth2/token",
		VaultURL:            "https://vault.example.com:8200",
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

var processingData = map[string]any{
	model.ParticipantIdentifier:         "participant-abc",
	"clientID.vaultAccess":              "client-123",
	"clientID.apiAccess":                "client-456",
	"cfm.participant.credentialservice": "https://example.com/credentialservice",
	"cfm.participant.protocolservice":   "https://example.com/protocolservice",
	"publicURL":                         "http://test.example.com:1234/fizz/buzz",
}

func TestEDCVActivityProcessor_Process_WithValidData(t *testing.T) {
	ih := MockIdentityHubClient{}
	processor := NewProcessor(validConfig(WithIdentityHub(ih)))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

func TestEDCVActivityProcessor_Process_MissingParticipantID(t *testing.T) {

	processor := NewProcessor(validConfig())
	ctx := context.Background()
	pd := copyOf(processingData)
	delete(pd, model.ParticipantIdentifier)
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "activity-1",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orchestration-1", activity, pd, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "error processing EDC-V activity")
}

func TestEDCVActivityProcessor_Process_MissingClientID(t *testing.T) {
	processor := NewProcessor(validConfig())
	ctx := context.Background()
	processingData := copyOf(processingData)
	delete(processingData, "clientID.vaultAccess")
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "activity-2",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orchestration-2", activity, processingData, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "error processing EDC-V activity")
}

func TestEDCVActivityProcessor_Process_MissingCredentialServiceUrl(t *testing.T) {
	processor := NewProcessor(validConfig())
	ctx := context.Background()
	processingData := copyOf(processingData)
	delete(processingData, "cfm.participant.credentialservice")
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "activity-2",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orchestration-2", activity, processingData, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "CredentialServiceURL is empty")
}

func TestEDCVActivityProcessor_Process_MissingProtocolServiceUrl(t *testing.T) {
	processor := NewProcessor(validConfig())
	ctx := context.Background()
	processingData := copyOf(processingData)
	delete(processingData, "cfm.participant.protocolservice")
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "activity-2",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orchestration-2", activity, processingData, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "ProtocolServiceURL is empty")
}

func TestEDCVActivityProcessor_Process_EmptyProcessingData(t *testing.T) {
	processor := NewProcessor(validConfig())
	ctx := context.Background()
	processingData := make(map[string]any)
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:   "activity-3",
		Type: "edcv",
	}

	activityContext := api.NewActivityContext(ctx, "orchestration-3", activity, processingData, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
}

func TestEDCVActivityProcessor_Process_InvalidDataTypes(t *testing.T) {
	processor := NewProcessor(validConfig())
	ctx := context.Background()
	pd := copyOf(processingData)
	pd["clientID.vaultAccess"] = 456      // Should be string
	pd[model.ParticipantIdentifier] = 123 // Should be string
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:   "activity-4",
		Type: "edcv",
	}

	activityContext := api.NewActivityContext(ctx, "orchestration-4", activity, pd, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
}

func TestEDCVActivityProcessor_Process_OrchestrationIDInError(t *testing.T) {
	processor := NewProcessor(validConfig())

	ctx := context.Background()
	pd := map[string]any{
		model.ParticipantIdentifier:         "participant-123",
		"cfm.participant.credentialservice": "https://example.com/credentialservice",
		"cfm.participant.protocolservice":   "https://example.com/protocolservice",
		"publicURL":                         "http://test.example.com:1234/fizz/buzz",
		// Missing clientIDs
	}
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:   "activity-5",
		Type: "edcv",
	}

	orchestrationID := "test-orch-12345"
	activityContext := api.NewActivityContext(ctx, orchestrationID, activity, pd, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	require.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), orchestrationID)
}

func TestEDCVActivityProcessor_Process_MultipleUnknownFields(t *testing.T) {
	ih := MockIdentityHubClient{}

	processor := NewProcessor(validConfig(WithIdentityHub(ih)))

	ctx := context.Background()
	pd := copyOf(processingData)
	pd["field1"] = "value1"
	pd["field2"] = "value2"
	pd["field3"] = "value3"
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "activity-multi",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-multi", activity, pd, outputData)

	result := processor.Process(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.Nil(t, result.Error)
}

func TestEDCVActivityProcessor_Process_MissingVaultEntry(t *testing.T) {

	ih := MockIdentityHubClient{}

	invalidConfig := validConfig(WithIdentityHub(ih))
	invalidConfig.VaultClient = NewMockVaultClient() // does not contain any secrets
	processor := NewProcessor(invalidConfig)

	ctx := context.Background()

	outputData := make(map[string]any)

	activity := api.Activity{
		ID:   "activity-multi",
		Type: "edcv",
	}

	activityContext := api.NewActivityContext(ctx, "orch-multi", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.NotNil(t, result.Error)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
}

func TestEDCVActivityProcessor_Process_IdentityHubFailure(t *testing.T) {
	ih := MockIdentityHubClient{expectedError: fmt.Errorf("some error")}
	processor := NewProcessor(validConfig(WithIdentityHub(ih)))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:   "test-activity",
		Type: "edcv",
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Error(t, result.Error, "some error")
}

func TestEDCVActivityProcessor_Process_ManagementAPIFailure(t *testing.T) {
	processor := NewProcessor(validConfig(WithControlPlane(MockManagementApiClient{expectedParticipantError: fmt.Errorf("some error")})))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
}

func TestEDCVActivityProcessor_Process_ManagementAPIFailureConfig(t *testing.T) {
	processor := NewProcessor(validConfig(WithControlPlane(MockManagementApiClient{expectedConfigError: fmt.Errorf("some error")})))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
}

func TestEDCVActivityProcessor_ProcessDispose(t *testing.T) {
	ih := MockIdentityHubClient{}
	processor := NewProcessor(validConfig(WithIdentityHub(ih)))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

func TestEDCVActivityProcessor_ProcessDispose_CtrlPlaneParticipantError(t *testing.T) {
	ih := MockIdentityHubClient{}
	processor := NewProcessor(validConfig(WithIdentityHub(ih), WithControlPlane(MockManagementApiClient{
		expectedParticipantError: fmt.Errorf("some error"),
	})))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	// expect to complete successfully, to unblock subsequent agents
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
}

func TestEDCVActivityProcessor_ProcessDispose_CtrlPlaneConfigError(t *testing.T) {
	ih := MockIdentityHubClient{}
	processor := NewProcessor(validConfig(WithIdentityHub(ih), WithControlPlane(MockManagementApiClient{
		expectedConfigError: fmt.Errorf("some error"),
	})))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	// expect to complete successfully, to unblock subsequent agents
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
}

func TestEDCVActivityProcessor_ProcessDispose_IdentityHubError(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedError: fmt.Errorf("some error"),
	}
	processor := NewProcessor(validConfig(WithIdentityHub(ih)))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	// expect to complete successfully, to unblock subsequent agents
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
}

type MockVaultClient struct {
	cache map[string]string
}

func NewMockVaultClient(secrets ...string) MockVaultClient {
	cache := make(map[string]string)
	for i := 0; i < len(secrets); i += 2 {
		if i+1 < len(secrets) {
			cache[secrets[i]] = secrets[i+1]
		}
	}
	return MockVaultClient{
		cache: cache,
	}
}

func (m MockVaultClient) ResolveSecret(ctx context.Context, path string) (string, error) {
	if value, ok := m.cache[path]; ok {
		return value, nil
	}
	return "", types.ErrNotFound
}

func (m MockVaultClient) StoreSecret(ctx context.Context, path string, value string) error {
	m.cache[path] = value
	return nil
}

func (m MockVaultClient) DeleteSecret(ctx context.Context, path string) error {
	delete(m.cache, path)
	return nil
}

func (m MockVaultClient) Close() error {
	return nil
}

func (m MockVaultClient) Health(ctx context.Context) error {
	return nil
}

func copyOf(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

type MockIdentityHubClient struct {
	expectedError       error
	expectedCredentials []identityhub.VerifiableCredentialResource
}

func (m MockIdentityHubClient) QueryCredentialByType(participantContextID string, credentialType string) ([]identityhub.VerifiableCredentialResource, error) {
	return m.expectedCredentials, m.expectedError
}

func (m MockIdentityHubClient) DeleteParticipantContext(participantContextID string) error {
	return m.expectedError
}

func (m MockIdentityHubClient) GetCredentialRequestState(string, string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockIdentityHubClient) RequestCredentials(string, identityhub.CredentialRequest) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockIdentityHubClient) CreateParticipantContext(identityhub.ParticipantManifest) (*identityhub.CreateParticipantContextResponse, error) {
	return &identityhub.CreateParticipantContextResponse{
		STSClientID:     "test-clientid",
		STSClientSecret: "test-secret-alias",
	}, m.expectedError
}

type MockManagementApiClient struct {
	expectedParticipantError error
	expectedConfigError      error
}

func (m MockManagementApiClient) DeleteConfig(participantContextID string) error {
	return m.expectedConfigError
}

func (m MockManagementApiClient) DeleteParticipantContext(participantContextID string) error {
	return m.expectedParticipantError
}

func (m MockManagementApiClient) CreateParticipantContext(controlplane.ParticipantContext) error {
	return m.expectedParticipantError
}

func (m MockManagementApiClient) CreateConfig(string, controlplane.ParticipantContextConfig) error {
	return m.expectedConfigError
}
