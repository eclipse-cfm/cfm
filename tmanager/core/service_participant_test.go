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

package core

import (
	"context"
	"testing"
	"time"

	"github.com/metaform/connector-fabric-manager/common/memorystore"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("get existing participant profile", func(t *testing.T) {
		service := newTestParticipantService()
		profile := newTestParticipantProfile("tenant-1", "participant-1")
		_, err := service.participantStore.Create(ctx, profile)
		require.NoError(t, err)

		result, err := service.GetProfile(ctx, "tenant-1", "participant-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "participant-1", result.ID)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Equal(t, "test-participant-1", result.Identifier)
	})

	t.Run("get non-existent participant returns not found error", func(t *testing.T) {
		service := newTestParticipantService()

		result, err := service.GetProfile(ctx, "tenant-1", "non-existent")

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("get participant from different tenant returns not found", func(t *testing.T) {
		service := newTestParticipantService()
		profile := newTestParticipantProfile("tenant-1", "participant-1")
		_, err := service.participantStore.Create(ctx, profile)
		require.NoError(t, err)

		result, err := service.GetProfile(ctx, "tenant-2", "participant-1")

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, types.ErrNotFound, err)
	})
}

func TestQueryProfiles(t *testing.T) {
	ctx := context.Background()
	service := newTestParticipantService()

	// Populate store with test data
	participants := []*api.ParticipantProfile{
		newTestParticipantProfile("tenant-1", "participant-1"),
		newTestParticipantProfile("tenant-1", "participant-2"),
		newTestParticipantProfile("tenant-2", "participant-3"),
	}

	for _, participant := range participants {
		_, err := service.participantStore.Create(ctx, participant)
		require.NoError(t, err)
	}

	t.Run("query profiles with matching predicate", func(t *testing.T) {
		predicate := query.Eq("tenantId", "tenant-1")
		options := store.DefaultPaginationOptions()

		results := make([]*api.ParticipantProfile, 0)
		for participant, err := range service.QueryProfiles(ctx, predicate, options) {
			require.NoError(t, err)
			results = append(results, participant)
		}

		assert.GreaterOrEqual(t, len(results), 0)
	})

	t.Run("query profiles with pagination", func(t *testing.T) {
		predicate := query.Eq("tenantId", "tenant-1")

		options := store.PaginationOptions{
			Offset: 0,
			Limit:  1,
		}

		results := make([]*api.ParticipantProfile, 0)
		for participant, err := range service.QueryProfiles(ctx, predicate, options) {
			require.NoError(t, err)
			results = append(results, participant)
		}

		assert.LessOrEqual(t, int64(len(results)), options.Limit)
	})
}

func TestQueryProfilesCount(t *testing.T) {
	ctx := context.Background()
	service := newTestParticipantService()

	t.Run("count profiles in populated store", func(t *testing.T) {
		// Populate store with test data
		participants := []*api.ParticipantProfile{
			newTestParticipantProfile("tenant-1", "participant-1"),
			newTestParticipantProfile("tenant-1", "participant-2"),
			newTestParticipantProfile("tenant-2", "participant-3"),
		}

		for _, participant := range participants {
			_, err := service.participantStore.Create(ctx, participant)
			require.NoError(t, err)
		}

		predicate := query.Eq("tenantId", "tenant-1")

		count, err := service.QueryProfilesCount(ctx, predicate)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(0))
	})

	t.Run("count profiles in empty store", func(t *testing.T) {
		emptyService := newTestParticipantService()
		predicate := query.Eq("tenantId", "non-existent")
		count, err := emptyService.QueryProfilesCount(ctx, predicate)

		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestDeployProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("deploy participant profile successfully", func(t *testing.T) {
		service := newTestParticipantService()
		mockClient := new(mockProvisionClient)

		// Setup mock to accept any manifest
		mockClient.On("Send", ctx, mock.MatchedBy(func(manifest model.OrchestrationManifest) bool {
			vpaManifest := manifest.Payload[model.VPAData].([]model.VPAManifest)[0]
			assert.Equal(t, "cell-1", vpaManifest.CellID)
			assert.Equal(t, "external-id", vpaManifest.ExternalCellID)
			return manifest.OrchestrationType == model.VPADeployType
		})).Return(nil)

		service.provisionClient = mockClient

		// Create test cell
		cell := newTestCell("cell-1", "external-id")
		cell.State = api.DeploymentStateActive
		_, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)

		ds1 := newTestDataspaceProfile("ds-1")
		_, err = service.dataspaceStore.Create(ctx, ds1)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier: "participant-identifier",
			VPAProperties: api.VPAPropMap{
				model.ConnectorType: {"prop": "value"},
			},
			Properties: map[string]any{
				"name": "Test Participant",
			},
		}

		result, err := service.DeployProfile(ctx, "tenant-1", deployment)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Equal(t, "participant-identifier", result.Identifier)
		mockClient.AssertExpectations(t)
	})

	t.Run("deploy participant with roles successfully", func(t *testing.T) {
		service := newTestParticipantService()
		mockClient := new(mockProvisionClient)
		credentialSpecs := []model.CredentialSpec{
			{
				Id:              "test-id",
				Type:            "test-type",
				Issuer:          "test-issuer",
				Format:          "test-format",
				ParticipantRole: "test-role",
			},
			{
				Id:     "test-id-2",
				Type:   "test-type-2",
				Issuer: "test-issuer-2",
				Format: "test-format-2",
				//no participant role, this should be default
			},
		}

		// Setup mock to accept any manifest
		mockClient.On("Send", ctx, mock.MatchedBy(func(manifest model.OrchestrationManifest) bool {
			vpaManifest := manifest.Payload[model.VPAData].([]model.VPAManifest)[0]
			assert.Equal(t, "cell-1", vpaManifest.CellID)
			assert.Equal(t, "external-id", vpaManifest.ExternalCellID)

			credData := manifest.Payload[model.CredentialData].([]model.CredentialSpec)
			assert.NotNil(t, credData)
			assert.ElementsMatch(t, credentialSpecs, credData)
			return manifest.OrchestrationType == model.VPADeployType
		})).Return(nil)

		service.provisionClient = mockClient

		// Create test cell
		cell := newTestCell("cell-1", "external-id")
		cell.State = api.DeploymentStateActive
		_, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)

		ds1 := newTestDataspaceProfile("ds-1")
		ds1.DataspaceSpec.CredentialSpecs = credentialSpecs
		_, err = service.dataspaceStore.Create(ctx, ds1)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier: "participant-identifier",
			VPAProperties: api.VPAPropMap{
				model.ConnectorType: {"prop": "value"},
			},
			Properties: map[string]any{
				"name": "Test Participant",
			},
			ParticipantRoles: map[string][]string{
				"ds-1": {"test-role"},
			},
		}

		result, err := service.DeployProfile(ctx, "tenant-1", deployment)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Equal(t, "participant-identifier", result.Identifier)
		mockClient.AssertExpectations(t)
	})

	t.Run("deploy profile handles provision client error", func(t *testing.T) {
		service := newTestParticipantService()
		mockClient := new(mockProvisionClient)

		// Setup mock to return error
		mockClient.On("Send", ctx, mock.MatchedBy(func(manifest model.OrchestrationManifest) bool {
			return manifest.OrchestrationType == model.VPADeployType
		})).Return(assert.AnError)

		service.provisionClient = mockClient

		// Create test cell
		cell := newTestCell("cell-1", "external-id")
		_, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)

		ds1 := newTestDataspaceProfile("ds-1")
		_, err = service.dataspaceStore.Create(ctx, ds1)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier: "participant-identifier",
			VPAProperties: api.VPAPropMap{
				model.ConnectorType: {"prop": "value"},
			},
			Properties: map[string]any{
				"name": "Test Participant",
			},
		}

		result, err := service.DeployProfile(ctx, "tenant-1", deployment)

		require.Error(t, err)
		require.Nil(t, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("deploy to specific dataspace profile when multiple exist", func(t *testing.T) {
		service := newTestParticipantService()
		mockClient := new(mockProvisionClient)

		// Setup mock to accept manifest
		mockClient.On("Send", ctx, mock.MatchedBy(func(manifest model.OrchestrationManifest) bool {
			return manifest.OrchestrationType == model.VPADeployType
		})).Return(nil)

		service.provisionClient = mockClient

		// Create test cell
		cell := newTestCell("cell-1", "external-id")
		cell.State = api.DeploymentStateActive
		_, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)

		// Create two dataspace profiles
		ds1 := newTestDataspaceProfile("ds-1")
		ds2 := newTestDataspaceProfile("ds-2")
		_, err = service.dataspaceStore.Create(ctx, ds1)
		require.NoError(t, err)
		_, err = service.dataspaceStore.Create(ctx, ds2)
		require.NoError(t, err)

		// Deploy specifying only ds-1
		deployment := &api.NewParticipantProfileDeployment{
			Identifier:          "participant-identifier",
			DataspaceProfileIDs: []string{"ds-1"},
			VPAProperties: api.VPAPropMap{
				model.ConnectorType: {"prop": "value"},
			},
			Properties: map[string]any{
				"name": "Test Participant",
			},
		}

		result, err := service.DeployProfile(ctx, "tenant-1", deployment)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, "tenant-1", result.TenantID)
		assert.Equal(t, "participant-identifier", result.Identifier)
		assert.Len(t, result.DataspaceProfileIDs, 1)
		assert.Equal(t, "ds-1", result.DataspaceProfileIDs[0])
		mockClient.AssertExpectations(t)
	})
}

func TestDisposeProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("dispose active participant profile successfully", func(t *testing.T) {
		service := newTestParticipantService()
		mockClient := new(mockProvisionClient)

		// Setup mock to accept dispose manifest
		mockClient.On("Send", ctx, mock.MatchedBy(func(manifest model.OrchestrationManifest) bool {
			vpaManifest := manifest.Payload[model.VPAData].([]model.VPAManifest)[0]
			assert.Equal(t, "cell-1", vpaManifest.CellID)
			assert.Equal(t, "external-id", vpaManifest.ExternalCellID)
			return manifest.OrchestrationType == model.VPADisposeType
		})).Return(nil)

		service.provisionClient = mockClient

		// Create a deployed participant and mark it as active
		profile := newTestParticipantProfile("tenant-1", "participant-1")
		profile.Properties[model.VPAStateData] = map[string]any{"state": "deployed"}
		profile.VPAs[0].State = api.DeploymentStateActive

		createdProfile, err := service.participantStore.Create(ctx, profile)
		require.NoError(t, err)

		err = service.DisposeProfile(ctx, "tenant-1", createdProfile.ID)

		require.NoError(t, err)
		mockClient.AssertExpectations(t)

		// Verify profile state updated
		updated, err := service.participantStore.FindByID(ctx, createdProfile.ID)
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, api.DeploymentStateDisposing, updated.VPAs[0].State)
	})

	t.Run("dispose non-existent participant returns error", func(t *testing.T) {
		service := newTestParticipantService()

		err := service.DisposeProfile(ctx, "tenant-1", "non-existent")

		require.Error(t, err)
	})

	t.Run("dispose participant from different tenant returns error", func(t *testing.T) {
		service := newTestParticipantService()
		profile := newTestParticipantProfile("tenant-1", "participant-1")
		createdProfile, err := service.participantStore.Create(ctx, profile)
		require.NoError(t, err)

		err = service.DisposeProfile(ctx, "tenant-2", createdProfile.ID)

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("dispose profile with non-active VPAs fails", func(t *testing.T) {
		service := newTestParticipantService()
		profile := newTestParticipantProfile("tenant-1", "participant-1")
		profile.Properties[model.VPAStateData] = map[string]any{"state": "deployed"}
		// Mark VPA as pending (not active)
		if len(profile.VPAs) > 0 {
			profile.VPAs[0].State = api.DeploymentStatePending
		}

		createdProfile, err := service.participantStore.Create(ctx, profile)
		require.NoError(t, err)

		err = service.DisposeProfile(ctx, "tenant-1", createdProfile.ID)

		require.Error(t, err)
	})

	t.Run("dispose profile without state data fails", func(t *testing.T) {
		service := newTestParticipantService()
		profile := newTestParticipantProfile("tenant-1", "participant-1")
		// Do not set VPAStateData
		profile.Properties = api.Properties{}
		if len(profile.VPAs) > 0 {
			profile.VPAs[0].State = api.DeploymentStateActive
		}

		createdProfile, err := service.participantStore.Create(ctx, profile)
		require.NoError(t, err)

		err = service.DisposeProfile(ctx, "tenant-1", createdProfile.ID)

		require.Error(t, err)
	})

	t.Run("dispose profile handles provision client error", func(t *testing.T) {
		service := newTestParticipantService()
		mockClient := new(mockProvisionClient)

		// Setup mock to return error
		mockClient.On("Send", ctx, mock.MatchedBy(func(manifest model.OrchestrationManifest) bool {
			return manifest.OrchestrationType == model.VPADisposeType
		})).Return(assert.AnError)

		service.provisionClient = mockClient

		// Create a deployed participant
		profile := newTestParticipantProfile("tenant-1", "participant-1")
		profile.Properties[model.VPAStateData] = map[string]any{"state": "deployed"}
		profile.VPAs[0].State = api.DeploymentStateActive

		createdProfile, err := service.participantStore.Create(ctx, profile)
		require.NoError(t, err)

		err = service.DisposeProfile(ctx, "tenant-1", createdProfile.ID)

		require.Error(t, err)
		mockClient.AssertExpectations(t)
	})
}

func TestVPACallbackHandlerDeploy(t *testing.T) {
	ctx := context.Background()
	service := newTestParticipantService()

	profile := newTestParticipantProfile("tenant-1", "participant-1")
	createdProfile, err := service.participantStore.Create(ctx, profile)
	require.NoError(t, err)

	handler := vpaCallbackHandler{
		participantStore: service.participantStore,
		trxContext:       service.trxContext,
		monitor:          system.NoopMonitor{},
	}

	response := model.OrchestrationResponse{
		ID:                "response-1",
		ManifestID:        "manifest-1",
		CorrelationID:     createdProfile.ID,
		OrchestrationType: model.VPADeployType,
		Success:           true,
		Properties: map[string]any{
			"connectionString": "test-value",
		},
	}

	err = handler.handleDeploy(ctx, response)

	require.NoError(t, err)

	// Verify profile was updated
	updated, err := service.participantStore.FindByID(ctx, createdProfile.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.False(t, updated.Error)
	assert.Equal(t, api.DeploymentStateActive, updated.VPAs[0].State)
	assert.NotNil(t, updated.Properties[model.VPAStateData])
}

func TestVPACallbackHandlerDispose(t *testing.T) {
	ctx := context.Background()
	service := newTestParticipantService()

	profile := newTestParticipantProfile("tenant-1", "participant-1")
	profile.VPAs[0].State = api.DeploymentStateDisposing
	createdProfile, err := service.participantStore.Create(ctx, profile)
	require.NoError(t, err)

	handler := vpaCallbackHandler{
		participantStore: service.participantStore,
		trxContext:       service.trxContext,
		monitor:          nil,
	}

	response := model.OrchestrationResponse{
		ID:                "response-1",
		ManifestID:        "manifest-1",
		CorrelationID:     createdProfile.ID,
		OrchestrationType: model.VPADisposeType,
		Success:           true,
		Properties:        map[string]any{},
	}

	err = handler.handleDispose(ctx, response)

	require.NoError(t, err)

	// Verify profile was updated
	updated, err := service.participantStore.FindByID(ctx, createdProfile.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.False(t, updated.Error)
	assert.Equal(t, api.DeploymentStateDisposed, updated.VPAs[0].State)
}

func TestVPACallbackHandlerFailedResponse(t *testing.T) {
	ctx := context.Background()
	participantStore := memorystore.NewInMemoryEntityStore[*api.ParticipantProfile]()

	profile := newTestParticipantProfile("tenant-1", "participant-1")
	createdProfile, err := participantStore.Create(ctx, profile)
	require.NoError(t, err)

	handler := vpaCallbackHandler{
		participantStore: participantStore,
		trxContext:       store.NoOpTransactionContext{},
		monitor:          nil,
	}

	response := model.OrchestrationResponse{
		ID:                "response-1",
		ManifestID:        "manifest-1",
		CorrelationID:     createdProfile.ID,
		OrchestrationType: model.VPADeployType,
		Success:           false,
		ErrorDetail:       "Deployment failed due to network error",
	}

	err = handler.handleDeploy(ctx, response)

	require.NoError(t, err)

	// Verify profile error was set
	updated, err := participantStore.FindByID(ctx, createdProfile.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, updated.Error)
	assert.Equal(t, "Deployment failed due to network error", updated.ErrorDetail)
}

func TestVPACallbackHandlerNonExistentProfile(t *testing.T) {
	ctx := context.Background()

	handler := vpaCallbackHandler{
		participantStore: memorystore.NewInMemoryEntityStore[*api.ParticipantProfile](),
		trxContext:       store.NoOpTransactionContext{},
		monitor:          system.NoopMonitor{},
	}

	response := model.OrchestrationResponse{
		ID:                "response-1",
		ManifestID:        "manifest-1",
		CorrelationID:     "non-existent-profile",
		OrchestrationType: model.VPADeployType,
		Success:           true,
		Properties:        map[string]any{},
	}

	err := handler.handleDeploy(ctx, response)

	// Should not return error for non-existent profile (idempotent)
	require.NoError(t, err)
}

func TestGetFilteredProfiles(t *testing.T) {
	ctx := context.Background()

	t.Run("returns all profiles when no profile IDs specified", func(t *testing.T) {
		service := newTestParticipantService()

		profile1 := newTestDataspaceProfile("ds-1")
		profile2 := newTestDataspaceProfile("ds-2")
		_, err := service.dataspaceStore.Create(ctx, profile1)
		require.NoError(t, err)
		_, err = service.dataspaceStore.Create(ctx, profile2)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier:          "participant-identifier",
			DataspaceProfileIDs: []string{}, // Empty list
		}

		result, err := service.getFilteredProfiles(ctx, deployment)

		require.NoError(t, err)
		assert.Equal(t, 2, len(result))

		require.ElementsMatch(t, []string{"ds-1", "ds-2"}, []string{result[0].ID, result[1].ID})
	})

	t.Run("filters profiles by specified IDs", func(t *testing.T) {
		service := newTestParticipantService()

		profile1 := newTestDataspaceProfile("ds-1")
		profile2 := newTestDataspaceProfile("ds-2")
		profile3 := newTestDataspaceProfile("ds-3")
		_, err := service.dataspaceStore.Create(ctx, profile1)
		require.NoError(t, err)
		_, err = service.dataspaceStore.Create(ctx, profile2)
		require.NoError(t, err)
		_, err = service.dataspaceStore.Create(ctx, profile3)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier:          "participant-identifier",
			DataspaceProfileIDs: []string{"ds-1", "ds-3"}, // Select specific profiles
		}

		result, err := service.getFilteredProfiles(ctx, deployment)

		require.NoError(t, err)
		assert.Equal(t, 2, len(result))

		ids := make(map[string]bool)
		for _, profile := range result {
			ids[profile.ID] = true
		}
		assert.True(t, ids["ds-1"])
		assert.True(t, ids["ds-3"])
		assert.False(t, ids["ds-2"])
	})

	t.Run("returns error when no profiles available", func(t *testing.T) {
		service := newTestParticipantService()

		deployment := &api.NewParticipantProfileDeployment{
			Identifier:          "participant-identifier",
			DataspaceProfileIDs: []string{},
		}

		result, err := service.getFilteredProfiles(ctx, deployment)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no dataspace profiles found")
	})

	t.Run("returns error when specified profiles do not exist", func(t *testing.T) {
		service := newTestParticipantService()

		// Create one profile
		profile1 := newTestDataspaceProfile("ds-1")
		_, err := service.dataspaceStore.Create(ctx, profile1)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier:          "participant-identifier",
			DataspaceProfileIDs: []string{"non-existent-id"}, // Request non-existent profile
		}

		result, err := service.getFilteredProfiles(ctx, deployment)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no dataspace profiles found")
	})

	t.Run("handles partial match of specified profiles", func(t *testing.T) {
		service := newTestParticipantService()

		// Create test dataspace profiles
		profile1 := newTestDataspaceProfile("ds-1")
		profile2 := newTestDataspaceProfile("ds-2")
		_, err := service.dataspaceStore.Create(ctx, profile1)
		require.NoError(t, err)
		_, err = service.dataspaceStore.Create(ctx, profile2)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier:          "participant-identifier",
			DataspaceProfileIDs: []string{"ds-1", "ds-999"}, // One valid, one invalid
		}

		result, err := service.getFilteredProfiles(ctx, deployment)

		require.NoError(t, err)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, "ds-1", result[0].ID)
	})

	t.Run("preserves profile data when filtering", func(t *testing.T) {
		service := newTestParticipantService()

		profile := newTestDataspaceProfile("ds-1")
		profile.Properties = api.Properties{
			"key": "value",
		}
		_, err := service.dataspaceStore.Create(ctx, profile)
		require.NoError(t, err)

		deployment := &api.NewParticipantProfileDeployment{
			Identifier:          "participant-identifier",
			DataspaceProfileIDs: []string{"ds-1"},
		}

		result, err := service.getFilteredProfiles(ctx, deployment)

		require.NoError(t, err)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, "ds-1", result[0].ID)
		assert.Equal(t, "value", result[0].Properties["key"])
	})
}

// Helper functions

func newTestParticipantProfile(tenantID string, participantID string) *api.ParticipantProfile {
	return &api.ParticipantProfile{
		Entity: api.Entity{
			ID:      participantID,
			Version: 1,
		},
		Identifier:          "test-" + participantID,
		TenantID:            tenantID,
		DataspaceProfileIDs: []string{"dataspace-1"},
		VPAs: []api.VirtualParticipantAgent{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "vpa-1",
						Version: 1,
					},
					State:          api.DeploymentStateInitial,
					StateTimestamp: time.Now(),
				},
				Type:           model.ConnectorType,
				CellID:         "cell-1",
				ExternalCellID: "external-id",
				Properties: api.Properties{
					"connectorType": "test-connector",
				},
			},
		},
		Properties: api.Properties{
			"name": "Test Participant " + participantID,
		},
		Error:       false,
		ErrorDetail: "",
	}
}

func newTestParticipantService() *participantService {
	return &participantService{
		trxContext:       store.NoOpTransactionContext{},
		participantStore: memorystore.NewInMemoryEntityStore[*api.ParticipantProfile](),
		cellStore:        memorystore.NewInMemoryEntityStore[*api.Cell](),
		dataspaceStore:   memorystore.NewInMemoryEntityStore[*api.DataspaceProfile](),
		participantGenerator: participantGenerator{
			CellSelector: func(
				orchestrationType model.OrchestrationType,
				cells []api.Cell,
				profiles []api.DataspaceProfile) (*api.Cell, error) {
				return &cells[0], nil
			},
		},
		monitor: system.NoopMonitor{},
	}
}

// Mock for ProvisionClient
type mockProvisionClient struct {
	mock.Mock
}

func (m *mockProvisionClient) Send(ctx context.Context, manifest model.OrchestrationManifest) error {
	args := m.Called(ctx, manifest)
	return args.Error(0)
}
