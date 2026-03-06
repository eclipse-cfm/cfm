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
	"strconv"
	"testing"

	"github.com/eclipse-cfm/cfm/common/memorystore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDataspaceProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("get existing dataspace profile", func(t *testing.T) {
		service := newTestDataspaceService()
		profile := newTestDataspaceProfile("dataspace-1")
		created, err := service.profileStore.Create(ctx, profile)
		require.NoError(t, err)

		result, err := service.GetProfile(ctx, "dataspace-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "dataspace-1", result.ID)
		assert.Equal(t, 1, len(result.Artifacts))
		assert.Equal(t, "artifact-1", result.Artifacts[0])
		assert.Equal(t, created.Version, result.Version)
	})

	t.Run("get non-existent dataspace profile returns not found error", func(t *testing.T) {
		service := newTestDataspaceService()

		result, err := service.GetProfile(ctx, "non-existent")

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, types.ErrNotFound, err)
	})
}

func TestCreateDataspaceProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("create dataspace profile with artifacts and properties", func(t *testing.T) {
		service := newTestDataspaceService()

		dprofile := &api.DataspaceProfile{
			Entity: api.Entity{
				ID:      uuid.New().String(),
				Version: 0,
			},
			DataspaceSpec: api.DataspaceSpec{},
			Artifacts:     []string{"artifact-1", "artifact-2"},
			Deployments:   nil,
			Properties: map[string]any{
				"name":        "Test Dataspace",
				"description": "A test dataspace profile",
			},
		}

		result, err := service.CreateProfile(ctx, dprofile)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.ID)
		assert.Equal(t, int64(0), result.Version)
		assert.Equal(t, dprofile.Artifacts, result.Artifacts)
		assert.Equal(t, len(dprofile.Properties), len(result.Properties))
		assert.Equal(t, "Test Dataspace", result.Properties["name"])
		assert.Equal(t, "A test dataspace profile", result.Properties["description"])
	})

	t.Run("create dataspace profile with empty artifacts", func(t *testing.T) {
		service := newTestDataspaceService()

		dprofile := &api.DataspaceProfile{
			Entity: api.Entity{
				ID:      uuid.New().String(),
				Version: 0,
			},
			DataspaceSpec: api.DataspaceSpec{},
			Artifacts:     []string{},
			Deployments:   nil,
			Properties: map[string]any{
				"name": "Empty Dataspace",
			},
		}

		result, err := service.CreateProfile(ctx, dprofile)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 0, len(result.Artifacts))
		assert.Equal(t, 0, len(result.Deployments))
	})

	t.Run("create dataspace profile with nil properties", func(t *testing.T) {
		service := newTestDataspaceService()

		dprofile := &api.DataspaceProfile{
			Entity: api.Entity{
				ID:      uuid.New().String(),
				Version: 0,
			},
			DataspaceSpec: api.DataspaceSpec{},
			Artifacts:     []string{"artifact-1"},
			Deployments:   nil,
		}

		result, err := service.CreateProfile(ctx, dprofile)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, len(result.Artifacts))
	})
}

func TestDeleteDataspaceProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("delete existing dataspace profile", func(t *testing.T) {
		service := newTestDataspaceService()
		profile := newTestDataspaceProfile("dataspace-delete-1")
		created, err := service.profileStore.Create(ctx, profile)
		require.NoError(t, err)
		require.NotNil(t, created)

		err = service.DeleteProfile(ctx, "dataspace-delete-1")

		require.NoError(t, err)

		// Verify profile is deleted
		_, err = service.GetProfile(ctx, "dataspace-delete-1")
		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("delete non-existent dataspace profile returns error", func(t *testing.T) {
		service := newTestDataspaceService()

		err := service.DeleteProfile(ctx, "non-existent")

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})
}

func TestDeployDataspaceProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("deploy profile to existing cell successfully", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create test cell
		cell := newTestCell("cell-1", "external-id")
		createdCell, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)
		require.NotNil(t, createdCell)

		// Create test profile
		profile := newTestDataspaceProfile("dataspace-1")
		createdProfile, err := service.profileStore.Create(ctx, profile)
		require.NoError(t, err)
		require.NotNil(t, createdProfile)

		// Deploy profile
		err = service.DeployProfile(ctx, "dataspace-1", "cell-1")

		require.NoError(t, err)

		// Verify deployment was added to profile
		updated, err := service.GetProfile(ctx, "dataspace-1")
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, 1, len(updated.Deployments))
		assert.Equal(t, "cell-1", updated.Deployments[0].CellID)
		assert.Equal(t, "external-id", updated.Deployments[0].ExternalCellID)
		assert.Equal(t, api.DeploymentStateActive, updated.Deployments[0].State)
	})

	t.Run("deploy profile to non-existent cell returns error", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create test profile
		profile := newTestDataspaceProfile("dataspace-1")
		_, err := service.profileStore.Create(ctx, profile)
		require.NoError(t, err)

		// Try to deploy to non-existent cell
		err = service.DeployProfile(ctx, "dataspace-1", "non-existent-cell")

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("deploy non-existent profile returns error", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create test cell
		cell := newTestCell("cell-1", "external-id")
		_, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)

		// Try to deploy non-existent profile
		err = service.DeployProfile(ctx, "non-existent-profile", "cell-1")

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("deploy profile multiple times to different cells", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create test cells
		cell1 := newTestCell("cell-1", "external-id")
		cell2 := newTestCell("cell-2", "external-id2")
		_, err := service.cellStore.Create(ctx, cell1)
		require.NoError(t, err)
		_, err = service.cellStore.Create(ctx, cell2)
		require.NoError(t, err)

		// Create test profile
		profile := newTestDataspaceProfile("dataspace-1")
		_, err = service.profileStore.Create(ctx, profile)
		require.NoError(t, err)

		// Deploy to first cell
		err = service.DeployProfile(ctx, "dataspace-1", "cell-1")
		require.NoError(t, err)

		// Deploy to second cell
		err = service.DeployProfile(ctx, "dataspace-1", "cell-2")
		require.NoError(t, err)

		// Verify both deployments exist
		updated, err := service.GetProfile(ctx, "dataspace-1")
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, 2, len(updated.Deployments))

		cellIDs := []string{updated.Deployments[0].CellID, updated.Deployments[1].CellID}
		assert.ElementsMatch(t, []string{"cell-1", "cell-2"}, cellIDs)
	})

	t.Run("deployment contains proper metadata", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create test cell and profile
		cell := newTestCell("cell-1", "external-id")
		_, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)

		profile := newTestDataspaceProfile("dataspace-1")
		_, err = service.profileStore.Create(ctx, profile)
		require.NoError(t, err)

		// Deploy profile
		err = service.DeployProfile(ctx, "dataspace-1", "cell-1")
		require.NoError(t, err)

		// Verify deployment metadata
		updated, err := service.GetProfile(ctx, "dataspace-1")
		require.NoError(t, err)
		require.NotNil(t, updated)
		require.Equal(t, 1, len(updated.Deployments))

		deployment := updated.Deployments[0]
		assert.NotEmpty(t, deployment.ID)
		assert.Equal(t, int64(0), deployment.Version)
		assert.Equal(t, api.DeploymentStateActive, deployment.State)
		assert.NotNil(t, deployment.Properties)
	})
}

func TestListDataspaceProfiles(t *testing.T) {
	ctx := context.Background()

	t.Run("list all profiles from populated store", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create multiple test profiles
		profiles := []*api.DataspaceProfile{
			newTestDataspaceProfile("dataspace-1"),
			newTestDataspaceProfile("dataspace-2"),
			newTestDataspaceProfile("dataspace-3"),
		}

		for i, profile := range profiles {
			profiles[i].ID = "dataspace-" + strconv.Itoa(i+1)
			_, err := service.profileStore.Create(ctx, profile)
			require.NoError(t, err)
		}

		results, err := service.ListProfiles(ctx)

		require.NoError(t, err)
		assert.Equal(t, 3, len(results))

		resultIDs := make([]string, len(results))
		for i, profile := range results {
			resultIDs[i] = profile.ID
		}
		assert.ElementsMatch(t, []string{"dataspace-1", "dataspace-2", "dataspace-3"}, resultIDs)
	})

	t.Run("list profiles from empty store", func(t *testing.T) {
		service := newTestDataspaceService()

		results, err := service.ListProfiles(ctx)

		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("list profiles returns all properties", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create profile with specific properties
		profile := newTestDataspaceProfile("dataspace-1")
		_, err := service.profileStore.Create(ctx, profile)
		require.NoError(t, err)

		results, err := service.ListProfiles(ctx)

		require.NoError(t, err)
		require.Equal(t, 1, len(results))

		result := results[0]
		assert.Equal(t, 1, len(result.Artifacts))
		assert.Equal(t, "artifact-1", result.Artifacts[0])
		assert.Equal(t, "Test Dataspace dataspace-1", result.Properties["name"])
	})

	t.Run("list profiles with deployments", func(t *testing.T) {
		service := newTestDataspaceService()

		// Create cell and profile
		cell := newTestCell("cell-1", "external-id")
		_, err := service.cellStore.Create(ctx, cell)
		require.NoError(t, err)

		profile := newTestDataspaceProfile("dataspace-1")
		_, err = service.profileStore.Create(ctx, profile)
		require.NoError(t, err)

		// Deploy profile
		err = service.DeployProfile(ctx, "dataspace-1", "cell-1")
		require.NoError(t, err)

		results, err := service.ListProfiles(ctx)

		require.NoError(t, err)
		require.Equal(t, 1, len(results))
		assert.Equal(t, 1, len(results[0].Deployments))
		assert.Equal(t, "cell-1", results[0].Deployments[0].CellID)
	})
}

// Helper functions

func newTestDataspaceProfile(id string) *api.DataspaceProfile {
	return &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      id,
			Version: 1,
		},
		Artifacts:   []string{"artifact-1"},
		Deployments: make([]api.DataspaceDeployment, 0),
		Properties: api.Properties{
			"name":        "Test Dataspace " + id,
			"description": "A test dataspace profile",
		},
	}
}

func newTestDataspaceService() *dataspaceProfileService {
	return &dataspaceProfileService{
		trxContext:   store.NoOpTransactionContext{},
		profileStore: memorystore.NewInMemoryEntityStore[*api.DataspaceProfile](),
		cellStore:    memorystore.NewInMemoryEntityStore[*api.Cell](),
	}
}
