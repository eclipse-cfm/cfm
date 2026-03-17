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
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCell(t *testing.T) {
	now := time.Now().UTC()

	t.Run("returns active cell with active deployment", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment1",
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: now,
						},
						CellID: "cell1",
					},
				},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "cell1", result.ID)
		assert.Equal(t, api.DeploymentStateActive, result.State)
	})

	t.Run("returns first matching active cell when multiple exist", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell2",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment1",
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: now,
						},
						CellID: "cell1",
					},
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment2",
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: now,
						},
						CellID: "cell2",
					},
				},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "cell1", result.ID) // Should return the first matching cell
	})

	t.Run("returns error when no active cells exist", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStatePending,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment1",
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: now,
						},
						CellID: "cell1",
					},
				},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, "no active cell found", err.Error())
	})

	t.Run("returns error when active cell has no active deployments", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment1",
								Version: 1,
							},
							State:          api.DeploymentStatePending,
							StateTimestamp: now,
						},
						CellID: "cell1",
					},
				},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, "no active cell found", err.Error())
	})

	t.Run("returns error when cells slice is empty", func(t *testing.T) {
		cells := []api.Cell{}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment1",
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: now,
						},
						CellID: "cell1",
					},
				},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, "no active cell found", err.Error())
	})

	t.Run("returns error when dataspace profiles slice is empty", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, "no active cell found", err.Error())
	})

	t.Run("returns error when dataspace profile has no deployments", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.Error(t, err)
		require.Nil(t, result)
		assert.Equal(t, "no active cell found", err.Error())
	})

	t.Run("handles multiple dataspace profiles with different deployment states", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment1",
								Version: 1,
							},
							State:          api.DeploymentStatePending,
							StateTimestamp: now,
						},
						CellID: "cell1",
					},
				},
			},
			{
				Entity: api.Entity{
					ID:      "profile2",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment2",
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: now,
						},
						CellID: "cell1",
					},
				},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "cell1", result.ID)
		assert.Equal(t, api.DeploymentStateActive, result.State)
	})

	t.Run("handles cells with different deployment states", func(t *testing.T) {
		cells := []api.Cell{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell1",
						Version: 1,
					},
					State:          api.DeploymentStatePending,
					StateTimestamp: now,
				},
			},
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell2",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: now,
				},
			},
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "cell3",
						Version: 1,
					},
					State:          api.DeploymentStateError,
					StateTimestamp: now,
				},
			},
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{
					ID:      "profile1",
					Version: 1,
				},
				Deployments: []api.DataspaceDeployment{
					{
						DeployableEntity: api.DeployableEntity{
							Entity: api.Entity{
								ID:      "deployment1",
								Version: 1,
							},
							State:          api.DeploymentStateActive,
							StateTimestamp: now,
						},
						CellID: "cell2",
					},
				},
			},
		}

		result, err := defaultCellSelector("test", cells, dProfiles)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "cell2", result.ID)
		assert.Equal(t, api.DeploymentStateActive, result.State)
	})
}
