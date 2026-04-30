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

	"github.com/eclipse-cfm/cfm/agent/common/issuerservice"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
)

func TestRegistrationActivityProcessor_MinimalValidData(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
		model.VPAData:               validVpaData(nil),
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
	assert.Equal(t, "did:web:someparticipant", issuerService.recorded.did)
	assert.Equal(t, "did:web:someparticipant", issuerService.recorded.name)
}

func TestRegistrationActivityProcessor_FullValidData(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	processingData := map[string]any{
		model.ParticipantIdentifier:  "did:web:someparticipant",
		"cfm.participant.holdername": "some holder",
		model.VPAData:                validVpaData(nil),
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
	assert.Equal(t, "did:web:someparticipant", issuerService.recorded.did)
	assert.Equal(t, "some holder", issuerService.recorded.name)
}

func TestRegistrationActivityProcessor_InvalidData(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	processingData := map[string]any{}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Error(t, result.Error)
}

func TestRegistrationActivityProcessor_IssuerServiceFails(t *testing.T) {
	issuerService := MockIssuerService{expectedError: fmt.Errorf("some error")}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
		model.VPAData:               validVpaData(nil),
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
}

func TestRegistrationActivityProcessor_VpaDataMissing(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
	}

	activityContext := api.NewActivityContext(context.Background(), "orch-123", api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}, processingData, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, model.VPAData)
}

func TestRegistrationActivityProcessor_VpaDataEmptySlice(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
		model.VPAData:               []any{},
	}

	activityContext := api.NewActivityContext(context.Background(), "orch-123", api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}, processingData, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, model.IssuerServiceType.String())
}

func TestRegistrationActivityProcessor_VpaDataTypeMismatch(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
		model.VPAData: []any{
			map[string]any{
				"vpaType": model.ConnectorType.String(),
			},
		},
	}

	activityContext := api.NewActivityContext(context.Background(), "orch-123", api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}, processingData, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, model.IssuerServiceType.String())
}

func TestRegistrationActivityProcessor_VpaDataPropertiesPassedToIssuerService(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	props := map[string]any{"region": "eu-west", "tier": "standard"}
	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
		model.VPAData:               validVpaData(props),
	}

	activityContext := api.NewActivityContext(context.Background(), "orch-123", api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}, processingData, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
	assert.Equal(t, props, issuerService.recorded.properties)
	assert.Equal(t, issuerService.recorded.properties, props)
}

func TestRegistrationActivityProcessor_VpaDataNoProperties(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
		model.VPAData: []any{
			map[string]any{
				"vpaType": model.IssuerServiceType.String(),
			},
		},
	}

	activityContext := api.NewActivityContext(context.Background(), "orch-123", api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DeployDiscriminator,
	}, processingData, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
	assert.Empty(t, issuerService.recorded.properties)
}

func TestRegistrationActivityProcessor_ProcessDispose(t *testing.T) {
	issuerService := MockIssuerService{}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

func TestRegistrationActivityProcessor_ProcessDispose_IssuerServiceFails(t *testing.T) {
	issuerService := MockIssuerService{expectedError: fmt.Errorf("some error")}
	processor := NewProcessor(&Config{
		LogMonitor:    system.NoopMonitor{},
		IssuerService: &issuerService,
	})

	ctx := context.Background()
	outputData := make(map[string]any)

	activity := api.Activity{
		ID:            "test-activity",
		Type:          "edcv",
		Discriminator: api.DisposeDiscriminator,
	}

	processingData := map[string]any{
		model.ParticipantIdentifier: "did:web:someparticipant",
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
}

// validVpaData returns a VPA data slice with a single issuer service entry.
// If properties is nil, the entry has no properties field.
func validVpaData(properties map[string]any) []any {
	entry := map[string]any{
		"vpaType": model.IssuerServiceType.String(),
	}
	if properties != nil {
		entry["properties"] = properties
	}
	return []any{entry}
}

type regData struct {
	did        string
	holderID   string
	name       string
	properties map[string]any
}

type MockIssuerService struct {
	expectedError error
	recorded      regData
}

func (m *MockIssuerService) QueryCredentialsByType(ctx context.Context, participantContextID string, credentialType string) ([]issuerservice.IssuerCredentialResourceDto, error) {
	return nil, nil
}

func (m *MockIssuerService) DeleteHolder(ctx context.Context, holderID string) error {
	return m.expectedError
}

func (m *MockIssuerService) RevokeCredential(ctx context.Context, participantContextID string, credentialID string) error {
	return nil
}

func (m *MockIssuerService) CreateHolder(ctx context.Context, did string, holderID string, name string, properties map[string]any) error {
	m.recorded.did = did
	m.recorded.holderID = holderID
	m.recorded.name = name
	m.recorded.properties = properties
	return m.expectedError
}
