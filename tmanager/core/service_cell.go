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

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

type cellService struct {
	trxContext       store.TransactionContext
	cellStore        store.EntityStore[*api.Cell]
	participantStore store.EntityStore[*api.ParticipantProfile]
	dataspaceStore   store.EntityStore[*api.DataspaceProfile]
}

func (d cellService) RecordExternalDeployment(ctx context.Context, cell *api.Cell) (*api.Cell, error) {
	return store.Trx[api.Cell](d.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.Cell, error) {
		if cell.ExternalID != "" {
			count, err := d.cellStore.CountByPredicate(ctx, query.Eq("externalId", cell.ExternalID))
			if err != nil {
				return nil, err
			}
			if count > 0 {
				return nil, types.ErrConflict
			}
		}
		return d.cellStore.Create(ctx, cell)
	})
}

// DeleteCell deletes a cell that no participant VPA and no dataspace profile deployment uses, apart from disposed ones.
func (t cellService) DeleteCell(ctx context.Context, cellID string) error {
	return t.trxContext.Execute(ctx, func(ctx context.Context) error {
		// The exclusive lock waits for deployments that share-locked the cell, so that the checks below see them, and
		// makes deployments that start later wait for this delete.
		if _, err := t.cellStore.FindByIDAndLock(ctx, cellID, store.LockExclusive); err != nil {
			return err
		}

		for profile, err := range t.participantStore.FindByPredicate(ctx, query.Eq("vpas.cellId", cellID)) {
			if err != nil {
				return err
			}
			if participantUsesCell(profile, cellID) {
				return types.NewClientError("cannot delete cell %s: participant profile %s has VPAs in it", cellID, profile.ID)
			}
		}

		for profile, err := range t.dataspaceStore.FindByPredicate(ctx, query.Eq("deployments.cellId", cellID)) {
			if err != nil {
				return err
			}
			if dataspaceProfileUsesCell(profile, cellID) {
				return types.NewClientError("cannot delete cell %s: dataspace profile %s is deployed to it", cellID, profile.ID)
			}
		}

		return t.cellStore.Delete(ctx, cellID)
	})
}

func (p cellService) ListCells(ctx context.Context) ([]api.Cell, error) {
	result := []api.Cell{}
	err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
		var err error
		result, err = collection.CollectAllDeref(p.cellStore.GetAll(ctx))
		return err
	})
	return result, err
}
