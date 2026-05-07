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

	"github.com/eclipse-cfm/cfm/agent/common"
	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"github.com/eclipse-cfm/cfm/agent/common/issuerservice"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultSchedule), result.Result)
	assert.NoError(t, result.Error)

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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

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

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultRetryError), result.Result)
	assert.ErrorContains(t, result.Error, "unexpected credential request state ")

	assert.Equal(t, "test-participant", activityContext.Values()["participantContextId"])
	assert.Contains(t, activityContext.Values(), "holderPid")
}

func TestOnboardingActivityProcessor_ProcessDispose(t *testing.T) {
	processor := OnboardingActivityProcessor{
		Monitor: system.NoopMonitor{},
		IssuerServiceApiClient: MockIssuerServiceApiClient{
			expectedCredentials: []issuerservice.IssuerCredentialResourceDto{
				{
					ID:                   "",
					ParticipantContextID: "test-participant",
					CredentialFormat:     common.CredentialFormat_VCDM20_COSE,
					VerifiableCredential: common.VerifiableCredential{},
				},
			},
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

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

func TestOnboardingActivityProcessor_ProcessDispose_RevocationFails(t *testing.T) {
	is := MockIssuerServiceApiClient{
		expectedCredentials: []issuerservice.IssuerCredentialResourceDto{
			{
				ID:                   "test-credential-id",
				ParticipantContextID: "test-participant",
				CredentialFormat:     common.CredentialFormat_VCDM20_COSE,
				VerifiableCredential: common.VerifiableCredential{},
			},
		},
		expectedError: fmt.Errorf("some error"),
	}
	processor := OnboardingActivityProcessor{
		Monitor:                system.NoopMonitor{},
		IdentityApiClient:      MockIdentityHubClient{},
		IssuerServiceApiClient: is,
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

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
}

func TestOnboardingActivityProcessor_ProcessDispose_NoCredentials(t *testing.T) {
	ih := MockIdentityHubClient{
		expectedError: fmt.Errorf("some error"),
	}
	processor := OnboardingActivityProcessor{
		Monitor:           system.NoopMonitor{},
		IdentityApiClient: ih,
		IssuerServiceApiClient: MockIssuerServiceApiClient{
			expectedCredentials: []issuerservice.IssuerCredentialResourceDto{},
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

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

type MockIdentityHubClient struct {
	expectedError       error
	expectedState       string
	expectedURL         string
	expectedCredentials []common.VerifiableCredentialResource
}

func (m MockIdentityHubClient) QueryCredentialByType(ctx context.Context, participantContextID string, credentialType string) ([]common.VerifiableCredentialResource, error) {
	return m.expectedCredentials, m.expectedError
}

func (m MockIdentityHubClient) DeleteParticipantContext(ctx context.Context, participantContextID string) error {
	//TODO implement me
	panic("implement me")
}

func (m MockIdentityHubClient) CreateParticipantContext(context.Context, identityhub.ParticipantManifest) (*identityhub.CreateParticipantContextResponse, error) {
	panic("not used here")
}

func (m MockIdentityHubClient) RequestCredentials(context.Context, string, identityhub.CredentialRequest) (string, error) {
	return m.expectedURL, m.expectedError
}

func (m MockIdentityHubClient) GetCredentialRequestState(context.Context, string, string) (string, error) {
	return m.expectedState, m.expectedError
}

type MockIssuerServiceApiClient struct {
	expectedError       error
	expectedCredentials []issuerservice.IssuerCredentialResourceDto
}

func (m MockIssuerServiceApiClient) QueryCredentialsByType(ctx context.Context, participantContextID string, credentialType string) ([]issuerservice.IssuerCredentialResourceDto, error) {
	return m.expectedCredentials, nil

}

func (m MockIssuerServiceApiClient) DeleteHolder(ctx context.Context, holderID string) error {
	//TODO implement me
	panic("implement me")
}

func (m MockIssuerServiceApiClient) CreateHolder(ctx context.Context, did string, holderID string, name string, properties map[string]any) error {
	return m.expectedError
}

func (m MockIssuerServiceApiClient) RevokeCredential(ctx context.Context, participantContextID string, credentialID string) error {
	return m.expectedError
}
