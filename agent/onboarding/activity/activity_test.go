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

	"github.com/metaform/connector-fabric-manager/agent/common/identityhub"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/stretchr/testify/assert"
)

func TestOnboardingActivityProcessor_Process_WhenNewRequest(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedURL: "https://example.com/credentialservice/request/123",
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess": "test-participant",
		"cfm.vpa.credentials": []any{
			map[string]string{
				"id":     "id",
				"format": "format",
				"issuer": "issuer",
				"type":   "type",
			},
		},
	}

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultSchedule), result.Result)
	assert.NoError(t, result.Error)

	assert.Equal(t, ih.expectedURL, activityContext.Values()["credentialRequest"])
	assert.Equal(t, "test-participant", activityContext.Values()["participantContextId"])
	assert.Contains(t, activityContext.Values(), "holderPid")
}

func TestOnboardingActivityProcessor_Process_WhenNewRequestError(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedError: fmt.Errorf("some error"),
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess": "test-participant",
		"cfm.vpa.credentials": []any{
			map[string]string{
				"id":     "id",
				"format": "format",
				"issuer": "issuer",
				"type":   "type",
			},
		},
	}

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
	assert.Empty(t, activityContext.OutputValues())
}

func TestOnboardingActivityProcessor_Process_WhenPendingRequestApiError(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedError: fmt.Errorf("some error"),
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess":   "test-participant",
		"participantContextId": "test-participant",
		"holderPid":            "test-holder-pid",
		"credentialRequest":    "https://example.com/credentialservice/request/123",
	}

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
	assert.Empty(t, activityContext.OutputValues())
}

func TestOnboardingActivityProcessor_Process_WhenPendingRequestIssued(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedState: identityhub.CredentialRequestStateIssued,
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess":   "test-participant",
		"participantContextId": "test-participant",
		"holderPid":            "test-holder-pid",
		"credentialRequest":    "https://example.com/credentialservice/request/123",
	}

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

	assert.Equal(t, "https://example.com/credentialservice/request/123", activityContext.OutputValues()["credentialRequest"])
	assert.Equal(t, "test-participant", activityContext.OutputValues()["participantContextId"])
	assert.Equal(t, "test-holder-pid", activityContext.OutputValues()["holderPid"])
}

func TestOnboardingActivityProcessor_Process_WhenPendingRequestCreated(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedState: identityhub.CredentialRequestStateCreated,
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess":   "test-participant",
		"participantContextId": "test-participant",
		"holderPid":            "test-holder-pid",
		"credentialRequest":    "https://example.com/credentialservice/request/123",
	}

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultSchedule), result.Result)
	assert.NoError(t, result.Error)

	assert.Empty(t, activityContext.OutputValues())
}

func TestOnboardingActivityProcessor_Process_WhenPendingRequestRejected(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedState: identityhub.CredentialRequestStateRejected,
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess":   "test-participant",
		"participantContextId": "test-participant",
		"holderPid":            "test-holder-pid",
		"credentialRequest":    "https://example.com/credentialservice/request/123",
	}

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
	assert.ErrorContains(t, result.Error, "credential request for participant 'test-participant' was rejected")

	assert.Empty(t, activityContext.OutputValues())
}

func TestOnboardingActivityProcessor_Process_WhenPendingRequestError(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedState: identityhub.CredentialRequestStateError,
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess":   "test-participant",
		"participantContextId": "test-participant",
		"holderPid":            "test-holder-pid",
		"credentialRequest":    "https://example.com/credentialservice/request/123",
	}

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
	assert.ErrorContains(t, result.Error, "credential request for participant 'test-participant' failed")

	assert.Empty(t, activityContext.OutputValues())
}

func TestOnboardingActivityProcessor_Process_WhenInvalidData(t *testing.T) {
	ih := MockIdentityHubClient{}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{}

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
	assert.Error(t, result.Error)
}

func TestOnboardingActivityProcessor_Process_WhenInvalidStateReceived(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedState: "invalid state foobar",
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
	}

	var processingData = map[string]any{
		"clientID.apiAccess":   "test-participant",
		"participantContextId": "test-participant",
		"holderPid":            "test-holder-pid",
		"credentialRequest":    "https://example.com/credentialservice/request/123",
	}

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultRetryError), result.Result)
	assert.ErrorContains(t, result.Error, "unexpected credential request state ")

	assert.Equal(t, "test-participant", activityContext.Values()["participantContextId"])
	assert.Contains(t, activityContext.Values(), "holderPid")
}

type MockIdentityHubClient struct {
	expectedError error
	expectedState string
	expectedURL   string
}

func (m MockIdentityHubClient) QueryCredentialByType(participantContextID string, credentialType string) ([]identityhub.VerifiableCredentialResource, error) {
	//TODO implement me
	panic("implement me")
}

func (m MockIdentityHubClient) DeleteParticipantContext(participantContextID string) error {
	//TODO implement me
	panic("implement me")
}

func (m MockIdentityHubClient) CreateParticipantContext(identityhub.ParticipantManifest) (*identityhub.CreateParticipantContextResponse, error) {
	panic("not used here")
}

func (m MockIdentityHubClient) RequestCredentials(string, identityhub.CredentialRequest) (string, error) {
	return m.expectedURL, m.expectedError
}

func (m MockIdentityHubClient) GetCredentialRequestState(string, string) (string, error) {
	return m.expectedState, m.expectedError
}
