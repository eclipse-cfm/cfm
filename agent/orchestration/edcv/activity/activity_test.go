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
	"testing"

	"github.com/eclipse-cfm/cfm/agent/orchestration/edcv/controlplane"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ConfigOptions func(*Config)

func WithControlPlane(client controlplane.ManagementAPIClient) ConfigOptions {
	return func(config *Config) {
		config.ManagementAPIClient = client
	}
}

func validConfig(opts ...ConfigOptions) *Config {
	c := Config{
		ManagementAPIClient: MockManagementApiClient{},
		LogMonitor:          system.NoopMonitor{},
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
	"participantContextId":              "client-456",
	"cfm.participant.credentialservice": "https://example.com/credentialservice",
	"cfm.participant.protocolservice":   "https://example.com/protocolservice",
	"publicURL":                         "http://test.example.com:1234/fizz/buzz",
	STSClientIDKey:                      "test-sts-client-id",
}

func TestEDCVActivityProcessor_Process_WithValidData(t *testing.T) {
	processor := NewProcessor(validConfig())

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, copyOf(processingData), outputData)

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), "error processing EDC-V activity")
}

func TestEDCVActivityProcessor_Process_MissingSTSClientID(t *testing.T) {
	processor := NewProcessor(validConfig())
	ctx := context.Background()
	pd := copyOf(processingData)
	delete(pd, STSClientIDKey)
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "activity-sts",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orchestration-sts", activity, pd, outputData)

	result := processor.ProcessDeploy(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), STSClientIDKey)
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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	require.NotNil(t, result.Error)
	assert.Contains(t, result.Error.Error(), orchestrationID)
}

func TestEDCVActivityProcessor_Process_MultipleUnknownFields(t *testing.T) {
	processor := NewProcessor(validConfig())

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

	result := processor.ProcessDeploy(activityContext)

	require.NotNil(t, result)
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.Nil(t, result.Error)
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

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, copyOf(processingData), outputData)

	result := processor.ProcessDeploy(activityContext)

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

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, copyOf(processingData), outputData)

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
}

func TestEDCVActivityProcessor_ProcessDispose(t *testing.T) {
	processor := NewProcessor(validConfig())

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, copyOf(processingData), outputData)

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

func TestEDCVActivityProcessor_ProcessDispose_CtrlPlaneParticipantError(t *testing.T) {
	processor := NewProcessor(validConfig(WithControlPlane(MockManagementApiClient{
		expectedParticipantError: fmt.Errorf("some error"),
	})))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, copyOf(processingData), outputData)

	result := processor.ProcessDispose(activityContext)

	// expect to complete successfully, to unblock subsequent agents
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
}

func TestEDCVActivityProcessor_ProcessDispose_CtrlPlaneConfigError(t *testing.T) {
	processor := NewProcessor(validConfig(WithControlPlane(MockManagementApiClient{
		expectedConfigError: fmt.Errorf("some error"),
	})))

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, copyOf(processingData), outputData)

	result := processor.ProcessDispose(activityContext)

	// expect to complete successfully, to unblock subsequent agents
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
}

func copyOf(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

type MockManagementApiClient struct {
	expectedParticipantError error
	expectedConfigError      error
}

func (m MockManagementApiClient) DeleteConfig(ctx context.Context, participantContextID string) error {
	return m.expectedConfigError
}

func (m MockManagementApiClient) DeleteParticipantContext(ctx context.Context, participantContextID string) error {
	return m.expectedParticipantError
}

func (m MockManagementApiClient) CreateParticipantContext(context.Context, controlplane.ParticipantContext) error {
	return m.expectedParticipantError
}

func (m MockManagementApiClient) CreateConfig(context.Context, string, controlplane.ParticipantContextConfig) error {
	return m.expectedConfigError
}
