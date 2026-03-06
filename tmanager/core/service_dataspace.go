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
	"time"

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/google/uuid"
)

type dataspaceProfileService struct {
	trxContext   store.TransactionContext
	profileStore store.EntityStore[*api.DataspaceProfile]
	cellStore    store.EntityStore[*api.Cell]
}

func (d dataspaceProfileService) GetProfile(ctx context.Context, profileID string) (*api.DataspaceProfile, error) {
	return store.Trx[api.DataspaceProfile](d.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.DataspaceProfile, error) {
		return d.profileStore.FindByID(ctx, profileID)
	})
}

func (d dataspaceProfileService) CreateProfile(ctx context.Context, profile *api.DataspaceProfile) (*api.DataspaceProfile, error) {
	return store.Trx[api.DataspaceProfile](d.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.DataspaceProfile, error) {
		return d.profileStore.Create(ctx, profile)
	})
}

func (t dataspaceProfileService) DeleteProfile(ctx context.Context, profileID string) error {
	return t.trxContext.Execute(ctx, func(ctx context.Context) error {
		return t.profileStore.Delete(ctx, profileID)
	})
}

func (d dataspaceProfileService) DeployProfile(ctx context.Context, profileID string, cellID string) error {
	return d.trxContext.Execute(ctx, func(ctx context.Context) error {
		profile, err := d.profileStore.FindByID(ctx, profileID)
		if err != nil {
			return err
		}

		cell, err := d.cellStore.FindByID(ctx, cellID)
		if err != nil {
			return err
		}

		// TODO validate not already deployed and handle deployment
		profile.Deployments = append(profile.Deployments, api.DataspaceDeployment{
			DeployableEntity: api.DeployableEntity{
				Entity: api.Entity{
					ID:      uuid.New().String(),
					Version: 0,
				},
				State:          api.DeploymentStateActive,
				StateTimestamp: time.Time{}.UTC(),
			},
			CellID:         cell.ID,
			ExternalCellID: cell.ExternalID,
			Properties:     make(map[string]any),
		})
		err = d.profileStore.Update(ctx, profile)
		if err != nil {
			return err
		}
		return nil

	})
}

func (d dataspaceProfileService) ListProfiles(ctx context.Context) ([]api.DataspaceProfile, error) {
	result := []api.DataspaceProfile{}
	err := d.trxContext.Execute(ctx, func(ctx context.Context) error {
		var err error
		result, err = collection.CollectAllDeref(d.profileStore.GetAll(ctx))
		return err
	})
	return result, err
}
