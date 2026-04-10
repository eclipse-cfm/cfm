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
	}

	activityContext := api.NewActivityContext(ctx, "orch-123", activity, processingData, outputData)

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "some error")
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

type regData struct {
	did      string
	holderID string
	name     string
}

type MockIssuerService struct {
	expectedError error
	recorded      regData
}

func (m *MockIssuerService) DeleteHolder(ctx context.Context, holderID string) error {
	return m.expectedError
}

func (m *MockIssuerService) RevokeCredential(ctx context.Context, participantContextID string, credentialID string) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockIssuerService) CreateHolder(ctx context.Context, did string, holderID string, name string) error {
	m.recorded.did = did
	m.recorded.holderID = holderID
	m.recorded.name = name
	return m.expectedError
}
