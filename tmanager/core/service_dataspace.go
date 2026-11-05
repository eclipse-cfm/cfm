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
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/google/uuid"
)

type dataspaceProfileService struct {
	trxContext       store.TransactionContext
	profileStore     store.EntityStore[*api.DataspaceProfile]
	cellStore        store.EntityStore[*api.Cell]
	participantStore store.EntityStore[*api.ParticipantProfile]
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

// DeleteProfile deletes a dataspace profile that no participant uses, apart from disposed participants.
func (t dataspaceProfileService) DeleteProfile(ctx context.Context, profileID string) error {
	return t.trxContext.Execute(ctx, func(ctx context.Context) error {
		// The exclusive lock waits for participant deployments that share-locked the profile, so that the check below
		// sees them, and makes deployments that start later wait for this delete.
		if _, err := t.profileStore.FindByIDAndLock(ctx, profileID, store.LockExclusive); err != nil {
			return err
		}

		for participant, err := range t.participantStore.FindByPredicate(ctx, query.Eq("dataspaceProfileIds", profileID)) {
			if err != nil {
				return err
			}
			if !participantIsDisposed(participant) {
				return types.NewClientError("cannot delete dataspace profile %s: participant profile %s uses it", profileID, participant.ID)
			}
		}

		return t.profileStore.Delete(ctx, profileID)
	})
}

func (d dataspaceProfileService) DeployProfile(ctx context.Context, profileID string, cellID string) error {
	return d.trxContext.Execute(ctx, func(ctx context.Context) error {
		profile, err := d.profileStore.FindByID(ctx, profileID)
		if err != nil {
			return err
		}

		// The shared lock keeps the cell from being deleted until the deployment is committed
		cell, err := d.cellStore.FindByIDAndLock(ctx, cellID, store.LockShared)
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
