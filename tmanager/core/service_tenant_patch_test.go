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

	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchTenant(t *testing.T) {
	ctx := context.Background()
	service := newTestTenantService()

	t.Run("patch tenant with add properties", func(t *testing.T) {
		// Setup: Create a tenant
		tenant := newTestTenant("tenant-1")
		created, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Execute: Add new properties
		props := map[string]any{
			"region":      "US-West",
			"environment": "production",
		}
		err = service.PatchTenant(ctx, "tenant-1", props, []string{})

		// Verify
		require.NoError(t, err)
		result, err := service.GetTenant(ctx, "tenant-1")
		require.NoError(t, err)
		assert.Equal(t, "US-West", result.Properties["region"])
		assert.Equal(t, "production", result.Properties["environment"])
		assert.Equal(t, "Test Tenant tenant-1", result.Properties["name"]) // Existing property preserved
		assert.Greater(t, result.Version, created.Version)                 // Version incremented
	})

	t.Run("patch tenant with update properties", func(t *testing.T) {
		// Setup: Create a tenant with properties
		tenant := &api.Tenant{
			Entity: api.Entity{ID: "tenant-2", Version: 1},
			Properties: api.Properties{
				"name":   "Original Name",
				"region": "US-East",
			},
		}
		_, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Execute: Update existing property
		props := map[string]any{
			"region": "US-West",
		}
		err = service.PatchTenant(ctx, "tenant-2", props, []string{})

		// Verify
		require.NoError(t, err)
		result, err := service.GetTenant(ctx, "tenant-2")
		require.NoError(t, err)
		assert.Equal(t, "US-West", result.Properties["region"])
		assert.Equal(t, "Original Name", result.Properties["name"]) // Other properties preserved
	})

	t.Run("patch tenant with delete properties (null values)", func(t *testing.T) {
		// Setup: Create a tenant with multiple properties
		tenant := &api.Tenant{
			Entity: api.Entity{ID: "tenant-3", Version: 1},
			Properties: api.Properties{
				"name":        "Test Tenant",
				"region":      "US-West",
				"environment": "production",
				"deprecated":  "value",
			},
		}
		_, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		err = service.PatchTenant(ctx, "tenant-3", map[string]any{}, []string{"deprecated"})

		// Verify
		require.NoError(t, err)
		result, err := service.GetTenant(ctx, "tenant-3")
		require.NoError(t, err)
		assert.Nil(t, result.Properties["deprecated"])
		assert.Equal(t, "Test Tenant", result.Properties["name"])
		assert.Equal(t, "US-West", result.Properties["region"])
	})

	t.Run("patch tenant with mixed operations", func(t *testing.T) {
		// Setup: Create a tenant
		tenant := &api.Tenant{
			Entity: api.Entity{ID: "tenant-4", Version: 1},
			Properties: api.Properties{
				"name":        "Original",
				"region":      "US-East",
				"old_setting": "value",
			},
		}
		_, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Execute: Add, update, and delete properties in one patch
		props := map[string]any{
			"name":        "Updated", // Update
			"environment": "staging", // Add
		}
		err = service.PatchTenant(ctx, "tenant-4", props, []string{"old_setting"})

		// Verify
		require.NoError(t, err)
		result, err := service.GetTenant(ctx, "tenant-4")
		require.NoError(t, err)
		assert.Equal(t, "Updated", result.Properties["name"])
		assert.Equal(t, "staging", result.Properties["environment"])
		assert.Nil(t, result.Properties["old_setting"])
		assert.Equal(t, "US-East", result.Properties["region"]) // Unmodified property preserved
	})

	t.Run("patch non-existent tenant returns error", func(t *testing.T) {
		props := map[string]any{
			"region": "US-West",
		}
		err := service.PatchTenant(ctx, "non-existent", props, []string{})

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})

	t.Run("patch tenant with empty diff", func(t *testing.T) {
		// Setup: Create a tenant
		tenant := newTestTenant("tenant-5")
		created, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Execute: Patch with empty diff
		props := make(map[string]any)
		err = service.PatchTenant(ctx, "tenant-5", props, []string{})

		// Verify
		require.NoError(t, err)
		result, err := service.GetTenant(ctx, "tenant-5")
		require.NoError(t, err)
		// Version may or may not change (implementation detail)
		assert.GreaterOrEqual(t, result.Version, created.Version)
	})

	t.Run("patch tenant preserves unmodified properties", func(t *testing.T) {
		// Setup: Create a tenant with many properties
		tenant := &api.Tenant{
			Entity: api.Entity{ID: "tenant-6", Version: 1},
			Properties: api.Properties{
				"property1": "value1",
				"property2": "value2",
				"property3": "value3",
				"property4": "value4",
			},
		}
		_, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Execute: Patch only one property
		props := map[string]any{
			"property2": "updated_value2",
		}
		err = service.PatchTenant(ctx, "tenant-6", props, []string{})

		// Verify
		require.NoError(t, err)
		result, err := service.GetTenant(ctx, "tenant-6")
		require.NoError(t, err)
		assert.Equal(t, "value1", result.Properties["property1"])
		assert.Equal(t, "updated_value2", result.Properties["property2"])
		assert.Equal(t, "value3", result.Properties["property3"])
		assert.Equal(t, "value4", result.Properties["property4"])
	})

	t.Run("patch tenant with complex values", func(t *testing.T) {
		// Setup: Create a tenant
		tenant := newTestTenant("tenant-7")
		_, err := service.CreateTenant(ctx, tenant)
		require.NoError(t, err)

		// Execute: Patch with complex types
		props := map[string]any{
			"tags":     []string{"tag1", "tag2"},
			"metadata": map[string]any{"key": "value"},
			"count":    42,
			"active":   true,
		}
		err = service.PatchTenant(ctx, "tenant-7", props, []string{})

		// Verify
		require.NoError(t, err)
		result, err := service.GetTenant(ctx, "tenant-7")
		require.NoError(t, err)
		assert.NotNil(t, result.Properties["tags"])
		assert.NotNil(t, result.Properties["metadata"])
		assert.Equal(t, 42.0, result.Properties["count"])
		assert.Equal(t, true, result.Properties["active"])
	})

}
