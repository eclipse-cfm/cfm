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

	"github.com/eclipse-cfm/cfm/common/memorystore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordExternalDeployment(t *testing.T) {
	ctx := context.Background()
	service := newTestCellService()
	cell := newTestCell("cell-1", "external-id")

	result, err := service.RecordExternalDeployment(ctx, cell)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "cell-1", result.ID)
	assert.Equal(t, int64(1), result.Version)
	assert.Equal(t, "external-id", result.ExternalID)
	assert.Equal(t, "Test Cell cell-1", result.Properties["name"])
}

func TestRecordExternalDeployment_DuplicateExternalId(t *testing.T) {
	ctx := context.Background()
	service := newTestCellService()
	cell := newTestCell("cell-1", "external-id")
	cell.ExternalID = "external-id"

	_, err := service.RecordExternalDeployment(ctx, cell)
	require.NoError(t, err)
	_, err = service.RecordExternalDeployment(ctx, cell)
	require.NotNil(t, err)
	require.ErrorAs(t, err, &types.ErrConflict)
}

func TestDeleteCell(t *testing.T) {

	t.Run("delete existing cell", func(t *testing.T) {
		ctx := context.Background()
		service := newTestCellService()
		cell := newTestCell("cell-delete-1", "external-id")
		created, err := service.RecordExternalDeployment(ctx, cell)
		require.NoError(t, err)
		require.NotNil(t, created)

		err = service.DeleteCell(ctx, "cell-delete-1")

		require.NoError(t, err)
	})

	t.Run("delete non-existent cell returns error", func(t *testing.T) {
		ctx := context.Background()
		service := newTestCellService()
		err := service.DeleteCell(ctx, "non-existent-cell")

		require.Error(t, err)
		assert.Equal(t, types.ErrNotFound, err)
	})
}

func TestListCells(t *testing.T) {

	t.Run("list all cells from populated store", func(t *testing.T) {
		ctx := context.Background()
		service := newTestCellService()

		// Create multiple test cells
		cells := []*api.Cell{
			newTestCell("cell-1", "external-id1"),
			newTestCell("cell-2", "external-id2"),
			newTestCell("cell-3", "external-id3"),
		}

		for _, cell := range cells {
			_, err := service.RecordExternalDeployment(ctx, cell)
			require.NoError(t, err)
		}

		results, err := service.ListCells(ctx)

		require.NoError(t, err)
		require.Equal(t, 3, len(results))

		expectedIDs := []string{"cell-1", "cell-2", "cell-3"}
		resultIDs := make([]string, len(results))
		for i, cell := range results {
			resultIDs[i] = cell.ID
		}
		assert.ElementsMatch(t, expectedIDs, resultIDs)
	})

	t.Run("list cells from empty store", func(t *testing.T) {
		ctx := context.Background()
		service := newTestCellService()

		results, err := service.ListCells(ctx)

		require.NoError(t, err)
		require.Equal(t, 0, len(results))
	})
}

func newTestCell(id string, externalId string) *api.Cell {
	return &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      id,
				Version: 1,
			},
			State:          api.DeploymentStateInitial,
			StateTimestamp: time.Now(),
		},
		ExternalID: externalId,
		Properties: api.Properties{
			"name": "Test Cell " + id,
		},
	}
}

func newTestCellService() *cellService {
	return &cellService{
		trxContext: store.NoOpTransactionContext{},
		cellStore:  memorystore.NewInMemoryEntityStore[*api.Cell](),
	}
}
