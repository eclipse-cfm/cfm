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

package v1alpha1

import (
	"fmt"
	"testing"
	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToParticipantProfile(t *testing.T) {
	testTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)

	input := &api.ParticipantProfile{
		Entity: api.Entity{
			ID:      "participant-123",
			Version: 1,
		},
		Identifier: "test-participant",
		TenantID:   "tenant-123",
		VPAs: []api.VirtualParticipantAgent{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "vpa-123",
						Version: 2,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: testTime,
				},
				Type:       model.ConnectorType,
				CellID:     "cell-123",
				Properties: api.Properties{"vpa-key": "vpa-value"},
			},
		},
		ParticipantRoles: map[string][]string{"foo": []string{"bar"}},
		Properties:       api.Properties{"participant-key": "participant-value"},
		Error:            true,
		ErrorDetail:      "error",
	}

	result := ToParticipantProfile(input)

	require.NotNil(t, result)
	assert.Equal(t, "participant-123", result.ID)
	assert.Equal(t, "tenant-123", result.TenantID)
	assert.Equal(t, int64(1), result.Version)
	assert.Equal(t, "test-participant", result.Identifier)
	assert.True(t, result.Error)
	assert.Equal(t, "error", result.ErrorDetail)
	assert.Equal(t, "bar", result.ParticipantRoles["foo"][0])
	assert.Len(t, result.VPAs, 1)
	assert.Equal(t, map[string]any{"participant-key": "participant-value"}, result.Properties)
}

func TestToVPACollection(t *testing.T) {
	testTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	input := &api.ParticipantProfile{
		VPAs: []api.VirtualParticipantAgent{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "vpa-1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: testTime,
				},
				Type:       model.ConnectorType,
				CellID:     "cell-1",
				Properties: api.Properties{"key1": "value1"},
			},
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "vpa-2",
						Version: 2,
					},
					State:          api.DeploymentStatePending,
					StateTimestamp: testTime,
				},
				Type:       model.CredentialServiceType,
				CellID:     "cell-2",
				Properties: api.Properties{"key2": "value2"},
			},
		},
	}

	result := ToVPACollection(input)

	require.Len(t, result, 2)

	// First VPA
	assert.Equal(t, "vpa-1", result[0].ID)
	assert.Equal(t, int64(1), result[0].Version)
	assert.Equal(t, "active", result[0].State)
	assert.Equal(t, testTime, result[0].StateTimestamp)
	assert.Equal(t, model.ConnectorType, result[0].Type)
	assert.Equal(t, "cell-1", result[0].CellID)
	assert.Equal(t, map[string]any{"key1": "value1"}, result[0].Properties)

	// Second VPA
	assert.Equal(t, "vpa-2", result[1].ID)
	assert.Equal(t, int64(2), result[1].Version)
	assert.Equal(t, "pending", result[1].State)
	assert.Equal(t, testTime, result[1].StateTimestamp)
	assert.Equal(t, model.CredentialServiceType, result[1].Type)
	assert.Equal(t, "cell-2", result[1].CellID)
	assert.Equal(t, map[string]any{"key2": "value2"}, result[1].Properties)
}

func TestToVPA(t *testing.T) {
	testTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.FixedZone("EST", -5*60*60))

	input := api.VirtualParticipantAgent{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "vpa-456",
				Version: 3,
			},
			State:          api.DeploymentStateError,
			StateTimestamp: testTime,
		},
		Type:       model.DataPlaneType,
		CellID:     "cell-456",
		Properties: api.Properties{"vpa-prop": "vpa-val"},
	}

	result := ToVPA(&input)

	require.NotNil(t, result)
	assert.Equal(t, "vpa-456", result.ID)
	assert.Equal(t, int64(3), result.Version)
	assert.Equal(t, "error", result.State)
	assert.Equal(t, testTime, result.StateTimestamp)
	assert.Equal(t, model.DataPlaneType, result.Type)
	assert.Equal(t, "cell-456", result.CellID)
	assert.Equal(t, map[string]any{"vpa-prop": "vpa-val"}, result.Properties)
}

func TestToCell(t *testing.T) {
	testTime := time.Date(2025, 6, 15, 10, 30, 45, 123456789, time.UTC)

	input := api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      "cell-789",
				Version: 5,
			},
			State:          api.DeploymentStateLocked,
			StateTimestamp: testTime,
		},
		Properties: api.Properties{
			"environment": "production",
			"region":      "us-west-2",
			"capacity":    100,
		},
		ExternalID: "external-id",
	}

	result := ToCell(&input)

	require.NotNil(t, result)
	assert.Equal(t, "cell-789", result.ID)
	assert.Equal(t, int64(5), result.Version)
	assert.Equal(t, "locked", result.State)
	assert.Equal(t, testTime, result.StateTimestamp)
	assert.Equal(t, "external-id", result.ExternalID)
	assert.Equal(t, map[string]any{
		"environment": "production",
		"region":      "us-west-2",
		"capacity":    100,
	}, result.Properties)
}

func TestToAPIParticipantProfile(t *testing.T) {
	testTime := time.Date(2025, 3, 10, 14, 25, 0, 0, time.Local)

	input := &ParticipantProfile{
		Entity: Entity{
			ID:      "api-participant-123",
			Version: 4,
		},
		Identifier: "api-test-participant",
		TenantID:   "api-tenant-123",
		VPAs: []VirtualParticipantAgent{
			{
				DeployableEntity: DeployableEntity{
					Entity: Entity{
						ID:      "api-vpa-123",
						Version: 1,
					},
					State:          "active",
					StateTimestamp: testTime,
				},
				Type:       model.ConnectorType,
				CellID:     "api-cell-123",
				Properties: map[string]any{"vpa-key": "vpa-value"},
			},
		},
		ParticipantRoles: map[string][]string{"foo": []string{"bar"}},
		Properties:       map[string]any{"participant-key": "participant-value"},
		Error:            true,
		ErrorDetail:      "error",
	}

	result := ToAPIParticipantProfile(input)

	require.NotNil(t, result)
	assert.Equal(t, "api-participant-123", result.ID)
	assert.Equal(t, "api-tenant-123", result.TenantID)
	assert.Equal(t, int64(4), result.Version)
	assert.Equal(t, "api-test-participant", result.Identifier)
	assert.Len(t, result.VPAs, 1)
	assert.Contains(t, result.Properties, "participant-key")
	assert.True(t, result.Error)
	assert.Equal(t, "bar", result.ParticipantRoles["foo"][0])
	assert.Equal(t, "error", result.ErrorDetail)
	assert.Equal(t, "participant-value", result.Properties["participant-key"])
}

func TestToAPIVPACollection(t *testing.T) {
	testTime := time.Date(2025, 2, 14, 8, 15, 30, 0, time.UTC)

	input := []VirtualParticipantAgent{
		{
			DeployableEntity: DeployableEntity{
				Entity: Entity{
					ID:      "vpa-collection-1",
					Version: 1,
				},
				State:          "pending",
				StateTimestamp: testTime,
			},
			Type:       model.ConnectorType,
			CellID:     "cell-collection-1",
			Properties: map[string]any{"prop1": "val1"},
		},
		{
			DeployableEntity: DeployableEntity{
				Entity: Entity{
					ID:      "vpa-collection-2",
					Version: 2,
				},
				State:          "offline",
				StateTimestamp: testTime,
			},
			Type:       model.DataPlaneType,
			CellID:     "cell-collection-2",
			Properties: map[string]any{"prop2": "val2"},
		},
	}

	result := ToAPIVPACollection(input)

	require.Len(t, result, 2)

	// First VPA
	assert.Equal(t, "vpa-collection-1", result[0].ID)
	assert.Equal(t, int64(1), result[0].Version)
	assert.Equal(t, api.DeploymentStatePending, result[0].State)
	assert.Equal(t, testTime.UTC(), result[0].StateTimestamp) // Should be converted to UTC
	assert.Equal(t, model.ConnectorType, result[0].Type)

	// Second VPA
	assert.Equal(t, "vpa-collection-2", result[1].ID)
	assert.Equal(t, int64(2), result[1].Version)
	assert.Equal(t, api.DeploymentStateOffline, result[1].State)
	assert.Equal(t, testTime.UTC(), result[1].StateTimestamp) // Should be converted to UTC
	assert.Equal(t, model.DataPlaneType, result[1].Type)
}

func TestToAPIVPA(t *testing.T) {
	// Test with non-UTC timezone to verify UTC conversion
	testTime := time.Date(2025, 4, 20, 16, 45, 0, 0, time.FixedZone("JST", 9*60*60))

	input := VirtualParticipantAgent{
		DeployableEntity: DeployableEntity{
			Entity: Entity{
				ID:      "api-vpa-456",
				Version: 6,
			},
			State:          "locked",
			StateTimestamp: testTime,
		},
		Type:       model.CredentialServiceType,
		CellID:     "api-cell-456",
		Properties: map[string]any{"vpa-env": "staging"},
	}

	result := ToAPIVPA(&input)

	require.NotNil(t, result)
	assert.Equal(t, "api-vpa-456", result.ID)
	assert.Equal(t, int64(6), result.Version)
	assert.Equal(t, api.DeploymentStateLocked, result.State)
	assert.Equal(t, testTime.UTC(), result.StateTimestamp)      // Should be converted to UTC
	assert.Equal(t, time.UTC, result.StateTimestamp.Location()) // Verify timezone is UTC
	assert.Equal(t, model.CredentialServiceType, result.Type)
	assert.Equal(t, "api-cell-456", result.CellID)
	assert.Contains(t, result.Properties, "vpa-env")
}

func TestToAPICell(t *testing.T) {
	// Test with different timezone to verify UTC conversion
	testTime := time.Date(2025, 7, 4, 9, 0, 0, 0, time.FixedZone("PST", -8*60*60))

	input := Cell{
		Entity: Entity{
			ID:      "api-cell-789",
			Version: 7,
		},
		NewCell: NewCell{
			State:          "error",
			StateTimestamp: testTime,
			Properties: map[string]any{
				"cluster":   "prod-cluster-1",
				"nodes":     5,
				"cpu_cores": 32,
				"memory_gb": 128,
			},
		},
	}

	result := ToAPICell(&input)

	require.NotNil(t, result)
	assert.Equal(t, "api-cell-789", result.ID)
	assert.Equal(t, int64(7), result.Version)
	assert.Equal(t, api.DeploymentStateError, result.State)
	assert.Equal(t, testTime.UTC(), result.StateTimestamp)      // Should be converted to UTC
	assert.Equal(t, time.UTC, result.StateTimestamp.Location()) // Verify timezone is UTC
	assert.Contains(t, result.Properties, "cluster")
	assert.Equal(t, "prod-cluster-1", result.Properties["cluster"])
	assert.Contains(t, result.Properties, "nodes")
	assert.Equal(t, 5, result.Properties["nodes"])
}

func TestNewAPICell(t *testing.T) {
	// Test with non-UTC timezone to verify UTC conversion
	testTime := time.Date(2025, 8, 12, 20, 30, 45, 123000000, time.FixedZone("CET", 1*60*60))

	input := NewCell{
		State:          "initial",
		StateTimestamp: testTime,
		ExternalID:     "externalId",
		Properties: map[string]any{
			"type":       "kubernetes",
			"version":    "1.28",
			"auto_scale": true,
		},
	}

	result := NewAPICell(&input)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)             // Should generate a new UUID
	assert.Equal(t, int64(0), result.Version) // Should be 0 for new cells
	assert.Equal(t, api.DeploymentStateInitial, result.State)
	assert.Equal(t, testTime.UTC(), result.StateTimestamp)      // Should be converted to UTC
	assert.Equal(t, time.UTC, result.StateTimestamp.Location()) // Verify timezone is UTC
	assert.Equal(t, "externalId", result.ExternalID)
	assert.Contains(t, result.Properties, "type")
	assert.Equal(t, "kubernetes", result.Properties["type"])
	assert.Contains(t, result.Properties, "auto_scale")
	assert.Equal(t, true, result.Properties["auto_scale"])
}

func TestTimestampUTCConversion(t *testing.T) {
	// Test various timezones to ensure they all convert to UTC properly
	timezones := []struct {
		name string
		zone *time.Location
	}{
		{"EST", time.FixedZone("EST", -5*60*60)},
		{"PST", time.FixedZone("PST", -8*60*60)},
		{"JST", time.FixedZone("JST", 9*60*60)},
		{"CET", time.FixedZone("CET", 1*60*60)},
		{"Local", time.Local},
		{"UTC", time.UTC},
	}

	baseTime := time.Date(2025, 5, 15, 12, 30, 45, 123456789, time.UTC)

	for _, tz := range timezones {
		t.Run(tz.name, func(t *testing.T) {
			testTime := baseTime.In(tz.zone)

			// Test ToAPIVPA
			vpaInput := VirtualParticipantAgent{
				DeployableEntity: DeployableEntity{
					Entity:         Entity{ID: "vpa", Version: 1},
					State:          "active",
					StateTimestamp: testTime,
				},
				Type:       model.ConnectorType,
				CellID:     "cell",
				Properties: map[string]any{},
			}

			vpaResult := ToAPIVPA(&vpaInput)
			assert.Equal(t, time.UTC, vpaResult.StateTimestamp.Location())
			assert.Equal(t, baseTime.UTC(), vpaResult.StateTimestamp)

			// Test ToAPICell
			cellInput := Cell{
				Entity: Entity{ID: "cell", Version: 1},
				NewCell: NewCell{
					State:          "active",
					StateTimestamp: testTime,
					Properties:     map[string]any{},
				},
			}

			cellResult := ToAPICell(&cellInput)
			assert.Equal(t, time.UTC, cellResult.StateTimestamp.Location())
			assert.Equal(t, baseTime.UTC(), cellResult.StateTimestamp)

			// Test NewAPICell
			newCellInput := NewCell{
				State:          "active",
				StateTimestamp: testTime,
				Properties:     map[string]any{},
			}

			newCellResult := NewAPICell(&newCellInput)
			assert.Equal(t, time.UTC, newCellResult.StateTimestamp.Location())
			assert.Equal(t, baseTime.UTC(), newCellResult.StateTimestamp)
		})
	}
}

func TestEmptyAndNilInputs(t *testing.T) {
	t.Run("ToParticipantProfile with nil", func(t *testing.T) {
		// This would panic, so we test with minimal valid input
		input := &api.ParticipantProfile{}
		result := ToParticipantProfile(input)
		require.NotNil(t, result)
		assert.Empty(t, result.ID)
		assert.Equal(t, int64(0), result.Version)
	})

	t.Run("ToVPACollection with empty VPAs", func(t *testing.T) {
		input := &api.ParticipantProfile{VPAs: []api.VirtualParticipantAgent{}}
		result := ToVPACollection(input)
		assert.Len(t, result, 0)
	})

	t.Run("ToAPIVPACollection with empty slice", func(t *testing.T) {
		result := ToAPIVPACollection([]VirtualParticipantAgent{})
		assert.Len(t, result, 0)
	})

	t.Run("Properties handling", func(t *testing.T) {
		// Test nil properties
		input := &api.ParticipantProfile{
			Properties: nil,
		}
		result := ToParticipantProfile(input)
		assert.Nil(t, result.Properties)

		// Test empty properties
		input.Properties = api.Properties{}
		result = ToParticipantProfile(input)
		assert.Empty(t, result.Properties)
	})
}

func TestNewAPICell_Defaults(t *testing.T) {
	before := time.Now().UTC()

	input := NewCell{
		ExternalID: "ext-1",
		Properties: map[string]any{"key": "value"},
	}

	result := NewAPICell(&input)

	after := time.Now().UTC()

	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, int64(0), result.Version)
	assert.Equal(t, api.DeploymentStateInitial, result.State)
	assert.Equal(t, "ext-1", result.ExternalID)
	assert.Contains(t, result.Properties, "key")

	// StateTimestamp should default to approximately now
	assert.False(t, result.StateTimestamp.IsZero(), "StateTimestamp should not be zero")
	assert.True(t, !result.StateTimestamp.Before(before), "StateTimestamp should be >= test start time")
	assert.True(t, !result.StateTimestamp.After(after), "StateTimestamp should be <= test end time")
}

func TestNewAPICell_ExplicitValues(t *testing.T) {
	testTime := time.Date(2025, 8, 12, 20, 30, 45, 123000000, time.FixedZone("CET", 1*60*60))

	input := NewCell{
		State:          "active",
		StateTimestamp: testTime,
		ExternalID:     "ext-2",
		Properties:     map[string]any{"key": "value"},
	}

	result := NewAPICell(&input)

	require.NotNil(t, result)
	assert.Equal(t, api.DeploymentStateActive, result.State)
	assert.Equal(t, testTime.UTC(), result.StateTimestamp)
}

func TestToDataspaceProfile(t *testing.T) {
	testTime := time.Date(2025, 1, 15, 10, 30, 45, 0, time.UTC)

	input := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "dataspace-profile-123",
			Version: 5,
		},
		Artifacts: []string{
			"artifact-1",
			"artifact-2",
			"artifact-3",
		},
		Deployments: []api.DataspaceDeployment{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "orchestration-1",
						Version: 2,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: testTime,
				},
				CellID:         "cell-1",
				ExternalCellID: "external-cell-1",
				Properties: api.Properties{
					"orchestration-env": "production",
					"replicas":          3,
				},
			},
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "orchestration-2",
						Version: 1,
					},
					State:          api.DeploymentStatePending,
					StateTimestamp: testTime,
				},
				CellID: "cell-2",
				Properties: api.Properties{
					"orchestration-env": "staging",
					"replicas":          1,
				},
			},
		},
		DataspaceSpec: api.DataspaceSpec{
			ProtocolStack: []string{"dsp-2025-1"},
			CredentialSpecs: []model.CredentialSpec{
				{
					Type:            "FooCredential",
					Issuer:          "did:web:bar.com",
					Format:          "VC1_0_JWT",
					ParticipantRole: "testrole",
				},
			},
		},
		Properties: api.Properties{
			"dataspace-name":   "TestDataspace",
			"protocol-version": "2025-1",
			"policy-version":   2,
		},
	}

	result := ToDataspaceProfile(input)

	require.NotNil(t, result)

	// Test Entity fields
	assert.Equal(t, "dataspace-profile-123", result.ID)
	assert.Equal(t, int64(5), result.Version)

	// Test spec
	assert.Len(t, result.DataspaceSpec.ProtocolStack, 1)
	assert.Equal(t, "dsp-2025-1", result.DataspaceSpec.ProtocolStack[0])
	assert.Len(t, result.DataspaceSpec.CredentialSpecs, 1)
	cspec := result.DataspaceSpec.CredentialSpecs[0]
	assert.Equal(t, "FooCredential", cspec.Type)
	assert.Equal(t, "did:web:bar.com", cspec.Issuer)
	assert.Equal(t, "VC1_0_JWT", cspec.Format)
	assert.Equal(t, "testrole", cspec.ParticipantRole)

	// Test Artifacts
	assert.Len(t, result.Artifacts, 3)
	assert.Equal(t, []string{
		"artifact-1",
		"artifact-2",
		"artifact-3",
	}, result.Artifacts)

	// Test Properties
	assert.Len(t, result.Properties, 3)
	assert.Equal(t, "TestDataspace", result.Properties["dataspace-name"])
	assert.Equal(t, "2025-1", result.Properties["protocol-version"])
	assert.Equal(t, 2, result.Properties["policy-version"])

	// Test Deployments
	assert.Len(t, result.Deployments, 2)

	// First deployment
	deployment1 := result.Deployments[0]
	assert.Equal(t, "orchestration-1", deployment1.ID)
	assert.Equal(t, int64(2), deployment1.Version)
	assert.Equal(t, "active", deployment1.State)
	assert.Equal(t, testTime.UTC(), deployment1.StateTimestamp)
	assert.Equal(t, time.UTC, deployment1.StateTimestamp.Location())
	assert.Equal(t, "cell-1", deployment1.CellID)
	assert.Equal(t, "external-cell-1", deployment1.ExternalCellID)
	assert.Equal(t, map[string]any{
		"orchestration-env": "production",
		"replicas":          3,
	}, deployment1.Properties)

	// Second deployment
	deployment2 := result.Deployments[1]
	assert.Equal(t, "orchestration-2", deployment2.ID)
	assert.Equal(t, int64(1), deployment2.Version)
	assert.Equal(t, "pending", deployment2.State)
	assert.Equal(t, testTime.UTC(), deployment2.StateTimestamp)
	assert.Equal(t, time.UTC, deployment2.StateTimestamp.Location())
	assert.Equal(t, "cell-2", deployment2.CellID)
	assert.Equal(t, map[string]any{
		"orchestration-env": "staging",
		"replicas":          1,
	}, deployment2.Properties)
}

func TestToDataspaceProfile_EmptyDeployments(t *testing.T) {
	input := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "dataspace-empty-deployments",
			Version: 1,
		},
		Artifacts:   []string{"artifact-1"},
		Deployments: []api.DataspaceDeployment{}, // Empty deployments
		Properties: api.Properties{
			"test-key": "test-value",
		},
	}

	result := ToDataspaceProfile(input)

	require.NotNil(t, result)
	assert.Equal(t, "dataspace-empty-deployments", result.ID)
	assert.Equal(t, int64(1), result.Version)
	assert.Len(t, result.Artifacts, 1)
	assert.Equal(t, "artifact-1", result.Artifacts[0])
	assert.Len(t, result.Deployments, 0)
	assert.Equal(t, "test-value", result.Properties["test-key"])
}

func TestToDataspaceProfile_NilAndEmptyValues(t *testing.T) {
	input := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "dataspace-minimal",
			Version: 0,
		},
		Artifacts:   nil, // nil artifacts
		Deployments: nil, // nil deployments
		Properties:  nil, // nil properties
	}

	result := ToDataspaceProfile(input)

	require.NotNil(t, result)
	assert.Equal(t, "dataspace-minimal", result.ID)
	assert.Equal(t, int64(0), result.Version)
	assert.Nil(t, result.Artifacts)
	assert.Len(t, result.Deployments, 0) // Should create empty slice
	assert.Nil(t, result.Properties)
}

func TestToDataspaceProfile_EmptyProperties(t *testing.T) {
	testTime := time.Date(2025, 3, 20, 14, 45, 0, 0, time.FixedZone("PST", -8*60*60))

	input := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "dataspace-empty-props",
			Version: 2,
		},
		Artifacts: []string{"artifact-1"},
		Deployments: []api.DataspaceDeployment{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "orchestration-empty-props",
						Version: 1,
					},
					State:          api.DeploymentStateInitial,
					StateTimestamp: testTime,
				},
				CellID:     "cell-empty-props",
				Properties: api.Properties{}, // Empty properties
			},
		},
		Properties: api.Properties{}, // Empty properties
	}

	result := ToDataspaceProfile(input)

	require.NotNil(t, result)
	assert.Equal(t, "dataspace-empty-props", result.ID)
	assert.Equal(t, int64(2), result.Version)
	assert.Len(t, result.Deployments, 1)
	assert.Equal(t, "orchestration-empty-props", result.Deployments[0].ID)
	assert.Equal(t, "cell-empty-props", result.Deployments[0].CellID)
	assert.Equal(t, testTime.UTC(), result.Deployments[0].StateTimestamp)
	assert.Equal(t, time.UTC, result.Deployments[0].StateTimestamp.Location())
	assert.Empty(t, result.Properties)
	assert.Empty(t, result.Deployments[0].Properties)
}

func TestToDataspaceProfile_MultipleArtifacts(t *testing.T) {
	input := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "dataspace-many-artifacts",
			Version: 3,
		},
		Artifacts: []string{
			"artifact-1",
			"artifact-2",
			"artifact-3",
			"artifact-4",
			"artifact-5",
			"artifact-6",
			"artifact-7",
		},
		Deployments: []api.DataspaceDeployment{},
		Properties: api.Properties{
			"artifact-count": 7,
			"version-range":  "v1-v2",
		},
	}

	result := ToDataspaceProfile(input)

	require.NotNil(t, result)
	assert.Equal(t, "dataspace-many-artifacts", result.ID)
	assert.Len(t, result.Artifacts, 7)
	assert.Contains(t, result.Artifacts, "artifact-1")
	assert.Contains(t, result.Artifacts, "artifact-2")
	assert.Contains(t, result.Artifacts, "artifact-3")
	assert.Contains(t, result.Artifacts, "artifact-4")
	assert.Contains(t, result.Artifacts, "artifact-5")
	assert.Contains(t, result.Artifacts, "artifact-6")
	assert.Contains(t, result.Artifacts, "artifact-7")
	assert.Equal(t, 7, result.Properties["artifact-count"])
}

func TestToDataspaceProfile_AllDeploymentStates(t *testing.T) {
	testTime := time.Date(2025, 2, 10, 9, 15, 30, 0, time.FixedZone("JST", 9*60*60))

	allStates := []api.DeploymentState{
		api.DeploymentStateInitial,
		api.DeploymentStatePending,
		api.DeploymentStateActive,
		api.DeploymentStateLocked,
		api.DeploymentStateOffline,
		api.DeploymentStateError,
	}

	expectedStateStrings := []string{
		"initial",
		"pending",
		"active",
		"locked",
		"offline",
		"error",
	}

	deployments := make([]api.DataspaceDeployment, len(allStates))
	for i, state := range allStates {
		deployments[i] = api.DataspaceDeployment{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      fmt.Sprintf("orchestration-%d", i),
					Version: int64(i + 1),
				},
				State:          state,
				StateTimestamp: testTime,
			},
			CellID: fmt.Sprintf("cell-%d", i),
			Properties: api.Properties{
				"state-test": true,
			},
		}
	}

	input := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "dataspace-all-states",
			Version: 1,
		},
		Artifacts:   []string{"artifact-1"},
		Deployments: deployments,
		Properties:  api.Properties{},
	}

	result := ToDataspaceProfile(input)

	require.NotNil(t, result)
	assert.Len(t, result.Deployments, len(allStates))

	for i, expectedState := range expectedStateStrings {
		deployment := result.Deployments[i]
		assert.Equal(t, fmt.Sprintf("orchestration-%d", i), deployment.ID)
		assert.Equal(t, int64(i+1), deployment.Version)
		assert.Equal(t, expectedState, deployment.State)
		assert.Equal(t, testTime.UTC(), deployment.StateTimestamp)
		assert.Equal(t, time.UTC, deployment.StateTimestamp.Location())
		assert.Equal(t, fmt.Sprintf("cell-%d", i), deployment.CellID)
		assert.Equal(t, true, deployment.Properties["state-test"])
	}
}

func TestToDataspaceProfile_ComplexProperties(t *testing.T) {
	testTime := time.Date(2025, 4, 25, 16, 20, 0, 0, time.FixedZone("CET", 1*60*60))

	input := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "dataspace-complex-props",
			Version: 10,
		},
		Artifacts: []string{"artifact-1"},
		Deployments: []api.DataspaceDeployment{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "complex-deployment",
						Version: 5,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: testTime,
				},
				CellID: "complex-cell",
				Properties: api.Properties{
					"config": map[string]any{
						"timeout":     30,
						"retry_count": 3,
						"endpoints":   []string{"http://api1.com", "http://api2.com"},
					},
					"metadata": map[string]any{
						"created_by": "test-system",
						"tags":       []string{"production", "critical"},
					},
				},
			},
		},
		Properties: api.Properties{
			"profile_config": map[string]any{
				"version":  "2.1.0",
				"features": []string{"feature1", "feature2", "feature3"},
				"limits":   map[string]any{"max_connections": 1000, "timeout_seconds": 60},
				"flags":    map[string]any{"enable_cache": true, "debug_mode": false},
			},
			"environment": "production",
			"owner":       "platform-team",
		},
	}

	result := ToDataspaceProfile(input)

	require.NotNil(t, result)
	assert.Equal(t, "dataspace-complex-props", result.ID)
	assert.Equal(t, int64(10), result.Version)

	// Test complex profile properties
	profileConfig, exists := result.Properties["profile_config"]
	require.True(t, exists)
	configMap, ok := profileConfig.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2.1.0", configMap["version"])

	features, exists := configMap["features"]
	require.True(t, exists)
	featuresSlice, ok := features.([]string)
	require.True(t, ok)
	assert.Len(t, featuresSlice, 3)
	assert.Contains(t, featuresSlice, "feature1")

	// Test deployment properties preservation and UTC conversion
	deployment := result.Deployments[0]
	assert.Equal(t, "complex-deployment", deployment.ID)
	assert.Equal(t, "complex-cell", deployment.CellID)
	assert.Equal(t, testTime.UTC(), deployment.StateTimestamp)
	assert.Equal(t, time.UTC, deployment.StateTimestamp.Location())

	config, exists := deployment.Properties["config"]
	require.True(t, exists)
	configDeployMap, ok := config.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 30, configDeployMap["timeout"])
	assert.Equal(t, 3, configDeployMap["retry_count"])
}

func TestToDataspaceProfile_TimestampUTCConversion(t *testing.T) {
	// Test different timezone handling - all should be converted to UTC
	baseTime := time.Date(2025, 6, 10, 12, 0, 0, 0, time.UTC)

	timezones := []struct {
		name string
		time time.Time
	}{
		{"UTC", baseTime.In(time.UTC)},
		{"EST", baseTime.In(time.FixedZone("EST", -5*60*60))},
		{"PST", baseTime.In(time.FixedZone("PST", -8*60*60))},
		{"JST", baseTime.In(time.FixedZone("JST", 9*60*60))},
		{"CET", baseTime.In(time.FixedZone("CET", 1*60*60))},
		{"Local", baseTime.In(time.Local)},
	}

	for _, tz := range timezones {
		t.Run(tz.name, func(t *testing.T) {
			input := &api.DataspaceProfile{
				Entity: api.Entity{
					ID:      "dataspace-" + tz.name,
					Version: 1,
				},
				Artifacts: []string{"artifact-1"},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "orchestration-" + tz.name,
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: tz.time,
						},
						CellID:     "cell-" + tz.name,
						Properties: api.Properties{},
					},
				},
				Properties: api.Properties{},
			}

			result := ToDataspaceProfile(input)

			require.NotNil(t, result)
			assert.Len(t, result.Deployments, 1)

			// Timestamps should be converted to UTC
			deployment := result.Deployments[0]
			assert.Equal(t, baseTime.UTC(), deployment.StateTimestamp)
			assert.Equal(t, time.UTC, deployment.StateTimestamp.Location())

			// Verify the time value is correct regardless of input timezone
			expectedUTCTime := tz.time.UTC()
			assert.Equal(t, expectedUTCTime, deployment.StateTimestamp)
		})
	}
}

func TestToTenant(t *testing.T) {
	input := &api.Tenant{
		Entity: api.Entity{
			ID:      "tenant-123",
			Version: 2,
		},
		Properties: api.Properties{"tenant-key": "tenant-value"},
	}

	result := ToTenant(input)

	require.NotNil(t, result)
	assert.Equal(t, "tenant-123", result.ID)
	assert.Equal(t, int64(2), result.Version)
	assert.Equal(t, map[string]any{"tenant-key": "tenant-value"}, result.Properties)
}

func TestToTenantNilProperties(t *testing.T) {
	input := &api.Tenant{
		Entity: api.Entity{
			ID:      "tenant-456",
			Version: 1,
		},
		Properties: nil,
	}

	result := ToTenant(input)

	require.NotNil(t, result)
	assert.Equal(t, "tenant-456", result.ID)
	assert.Nil(t, result.Properties)
}

func TestNewAPITenant(t *testing.T) {
	input := &NewTenant{
		Properties: map[string]any{"new-key": "new-value"},
	}

	result := NewAPITenant(input)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, int64(0), result.Version)
	assert.Contains(t, result.Properties, "new-key")
}

func TestNewAPITenantNilProperties(t *testing.T) {
	input := &NewTenant{
		Properties: nil,
	}

	result := NewAPITenant(input)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, int64(0), result.Version)
	assert.NotNil(t, result.Properties)
}

func TestNewAPIDataspaceProfile(t *testing.T) {
	input := &NewDataspaceProfile{
		DataspaceSpec: DataspaceSpec{
			ProtocolStack: []string{"dsp-2025-1"},
			CredentialSpecs: []CredentialSpec{
				{
					Type:            "FooCredential",
					Issuer:          "did:web:bar.com",
					Format:          "VC1_0_JWT",
					ParticipantRole: "testrole",
				},
			},
		},
		Artifacts: []string{"artifact-1", "artifact-2"},
		Properties: map[string]any{
			"key1": "value1",
		},
	}

	result := NewAPIDataspaceProfile(input)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, int64(0), result.Version)

	// Verify Spec
	assert.Equal(t, input.DataspaceSpec.ProtocolStack, result.DataspaceSpec.ProtocolStack)
	require.Len(t, result.DataspaceSpec.CredentialSpecs, 1)
	assert.Equal(t, input.DataspaceSpec.CredentialSpecs[0].Type, result.DataspaceSpec.CredentialSpecs[0].Type)
	assert.Equal(t, input.DataspaceSpec.CredentialSpecs[0].Issuer, result.DataspaceSpec.CredentialSpecs[0].Issuer)
	assert.Equal(t, input.DataspaceSpec.CredentialSpecs[0].Format, result.DataspaceSpec.CredentialSpecs[0].Format)
	assert.Equal(t, input.DataspaceSpec.CredentialSpecs[0].ParticipantRole, result.DataspaceSpec.CredentialSpecs[0].ParticipantRole)

	// Verify Artifacts
	assert.Equal(t, input.Artifacts, result.Artifacts)

	// Verify Deployments (should be empty but not nil as per implementation)
	assert.NotNil(t, result.Deployments)
	assert.Len(t, result.Deployments, 0)

	// Verify Properties
	assert.Equal(t, "value1", result.Properties["key1"])
}

func TestToAPINewParticipantProfileDeployment(t *testing.T) {
	input := &NewParticipantProfileDeployment{
		Identifier:          "test-participant",
		CellID:              "cell-123",
		DataspaceProfileIDs: []string{"profile-1", "profile-2"},
		ParticipantRoles: map[string][]string{
			"role1": {"permission1", "permission2"},
			"role2": {"permission3"},
		},
		VPAProperties: map[string]map[string]any{
			"vpa-1": {"key1": "value1"},
			"vpa-2": {"key2": "value2"},
		},
		Properties: map[string]any{
			"custom-key": "custom-value",
		},
	}

	result := ToAPINewParticipantProfileDeployment(input)

	require.NotNil(t, result)
	assert.Equal(t, "test-participant", result.Identifier)
	assert.Equal(t, "cell-123", result.CellID)
	assert.Len(t, result.DataspaceProfileIDs, 2)
	assert.Contains(t, result.DataspaceProfileIDs, "profile-1")
	assert.Contains(t, result.DataspaceProfileIDs, "profile-2")
	assert.Len(t, result.ParticipantRoles, 2)
	assert.Equal(t, []string{"permission1", "permission2"}, result.ParticipantRoles["role1"])
	assert.Len(t, result.VPAProperties, 2)
	assert.Equal(t, "value1", result.VPAProperties["vpa-1"]["key1"])
	assert.Equal(t, "custom-value", result.Properties["custom-key"])
}

func TestToAPINewParticipantProfileDeployment_NilValuesHandling(t *testing.T) {
	input := &NewParticipantProfileDeployment{
		Identifier:          "did:web:example.com",
		CellID:              "cell-123",
		DataspaceProfileIDs: nil, // nil slice
		ParticipantRoles:    nil, // nil map
		VPAProperties:       nil, // nil map
		Properties:          nil, // nil map
	}

	result := ToAPINewParticipantProfileDeployment(input)

	require.NotNil(t, result)
	assert.Equal(t, "did:web:example.com", result.Identifier)
	assert.Equal(t, "cell-123", result.CellID)

	// Verify nil values are converted to empty collections
	assert.NotNil(t, result.DataspaceProfileIDs, "DataspaceProfileIDs should not be nil")
	assert.Len(t, result.DataspaceProfileIDs, 0, "DataspaceProfileIDs should be empty slice")

	assert.NotNil(t, result.ParticipantRoles, "ParticipantRoles should not be nil")
	assert.Len(t, result.ParticipantRoles, 0, "ParticipantRoles should be empty map")

	assert.NotNil(t, result.VPAProperties, "VPAProperties should not be nil")
	assert.Len(t, result.VPAProperties, 0, "VPAProperties should be empty map")

	assert.NotNil(t, result.Properties, "Properties should not be nil")
	assert.Len(t, result.Properties, 0, "Properties should be empty map")
}
