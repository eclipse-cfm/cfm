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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenNewRequest(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenNewRequestError(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenPendingRequestApiError(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenPendingRequestIssued(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenPendingRequestCreated(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenPendingRequestRejected(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenPendingRequestError(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenInvalidData(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDeploy_WhenInvalidStateReceived(t *testing.T) {
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

func TestOnboardingActivityProcessor_ProcessDispose(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedCredentials: []identityhub.VerifiableCredentialResource{
			{
				ParticipantContextID: "test-participant",
				IssuerID:             "test-issuer",
				HolderID:             "test-participant",
				Metadata:             nil,
				State:                0,
				VerifiableCredential: identityhub.CredentialContainer{},
			},
		},
	}
	processor := OnboardingActivityProcessor{
		Monitor:                system.NoopMonitor{},
		IdentityApiClient:      ih,
		IssuerServiceApiClient: MockIssuerServiceApiClient{},
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
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}
func TestOnboardingActivityProcessor_ProcessDispose_RevocationFails(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedCredentials: []identityhub.VerifiableCredentialResource{
			{
				ParticipantContextID: "test-participant",
				IssuerID:             "test-issuer",
				HolderID:             "test-participant",
				Metadata:             nil,
				State:                0,
				VerifiableCredential: identityhub.CredentialContainer{},
			},
		},
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
		IssuerServiceApiClient: MockIssuerServiceApiClient{
			expectedError: fmt.Errorf("some error"),
		},
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
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
}

func TestOnboardingActivityProcessor_ProcessDispose_NoCredentials(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedCredentials: []identityhub.VerifiableCredentialResource{},
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
		IssuerServiceApiClient: MockIssuerServiceApiClient{
			expectedError: fmt.Errorf("some error"), // should never be invoked, because there are no credentials
		},
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
		Discriminator: api.DisposeDiscriminator,
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.Process(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

type MockIdentityHubClient struct {
	expectedError       error
	expectedState       string
	expectedURL         string
	expectedCredentials []identityhub.VerifiableCredentialResource
}

func (m MockIdentityHubClient) QueryCredentialByType(participantContextID string, credentialType string) ([]identityhub.VerifiableCredentialResource, error) {
	return m.expectedCredentials, m.expectedError
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

type MockIssuerServiceApiClient struct {
	expectedError error
}

func (m MockIssuerServiceApiClient) DeleteHolder(holderID string) error {
	//TODO implement me
	panic("implement me")
}

func (m MockIssuerServiceApiClient) CreateHolder(did string, holderID string, name string) error {
	return m.expectedError
}

func (m MockIssuerServiceApiClient) RevokeCredential(participantContextID string, credentialID string) error {
	return m.expectedError
}
