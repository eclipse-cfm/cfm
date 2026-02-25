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
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/google/uuid"
	"github.com/metaform/connector-fabric-manager/common/collection"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/tmanager/api"
)

type participantService struct {
	participantGenerator participantGenerator
	provisionClient      api.ProvisionClient
	trxContext           store.TransactionContext
	participantStore     store.EntityStore[*api.ParticipantProfile]
	cellStore            store.EntityStore[*api.Cell]
	dataspaceStore       store.EntityStore[*api.DataspaceProfile]
	monitor              system.LogMonitor
}

func (p participantService) GetProfile(ctx context.Context, tenantID string, participantID string) (*api.ParticipantProfile, error) {
	return store.Trx[api.ParticipantProfile](p.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.ParticipantProfile, error) {
		profile, err := p.participantStore.FindByID(ctx, participantID)
		if err != nil {
			return nil, err
		}
		if profile.TenantID != tenantID {
			return nil, types.ErrNotFound
		}
		return profile, nil
	})
}

func (p participantService) QueryProfilesCount(ctx context.Context, predicate query.Predicate) (int64, error) {
	var count int64
	err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
		c, err := p.participantStore.CountByPredicate(ctx, predicate)
		count = c
		return err
	})
	return count, err
}

func (p participantService) QueryProfiles(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.ParticipantProfile, error] {
	return p.executeStoreIterator(ctx, func(ctx context.Context) iter.Seq2[*api.ParticipantProfile, error] {
		return p.participantStore.FindByPredicatePaginated(ctx, predicate, options)
	})
}

func (p participantService) DeployProfile(
	ctx context.Context,
	tenantID string,
	deployment *api.NewParticipantProfileDeployment) (*api.ParticipantProfile, error) {

	// TODO perform property validation against a custom schema
	return store.Trx[api.ParticipantProfile](p.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.ParticipantProfile, error) {
		cells, err := collection.CollectAllDeref(p.cellStore.GetAll(ctx))
		if err != nil {
			return nil, err
		}

		dProfiles, err := p.getFilteredProfiles(ctx, deployment)
		if err != nil {
			return nil, err
		}

		participantProfile, err := p.participantGenerator.Generate(
			deployment.Identifier,
			tenantID,
			deployment.ParticipantRoles,
			deployment.VPAProperties,
			deployment.Properties,
			cells,
			dProfiles)
		if err != nil {
			return nil, err
		}

		oManifest := model.OrchestrationManifest{
			ID:                uuid.New().String(),
			CorrelationID:     participantProfile.ID,
			OrchestrationType: model.VPADeployType,
			Payload:           make(map[string]any),
		}

		oManifest.Payload[model.ParticipantIdentifier] = participantProfile.Identifier

		vpaManifests := make([]model.VPAManifest, 0, len(participantProfile.VPAs))
		for _, vpa := range participantProfile.VPAs {
			vpaManifest := model.VPAManifest{
				ID:             vpa.ID,
				VPAType:        vpa.Type,
				CellID:         vpa.CellID,
				ExternalCellID: vpa.ExternalCellID,
				Properties:     vpa.Properties,
			}
			vpaManifests = append(vpaManifests, vpaManifest)
		}
		oManifest.Payload[model.VPAData] = vpaManifests

		specs := generateCredentialSpecs(participantProfile.ParticipantRoles, dProfiles)
		oManifest.Payload[model.CredentialData] = specs

		result, err := p.participantStore.Create(ctx, participantProfile)
		if err != nil {
			return nil, fmt.Errorf("error creating participant %s: %w", deployment.Identifier, err)
		}

		// Only send the orchestration message if the storage operation succeeded. If the send fails, the transaction
		// will be rolled back.
		err = p.provisionClient.Send(ctx, oManifest)
		if err != nil {
			return nil, fmt.Errorf("error deploying participant %s: %w", deployment.Identifier, err)
		}

		return result, nil
	})
}

// getFilteredProfiles filters dProfiles based on deployment.DataspaceProfileIDs
func (p participantService) getFilteredProfiles(
	ctx context.Context,
	deployment *api.NewParticipantProfileDeployment) ([]api.DataspaceProfile, error) {

	dProfiles, err := collection.CollectAllDeref(p.dataspaceStore.GetAll(ctx))
	if err != nil {
		return nil, err
	}

	if len(deployment.DataspaceProfileIDs) > 0 {
		profileIDMap := make(map[string]bool)
		for _, id := range deployment.DataspaceProfileIDs {
			profileIDMap[id] = true
		}
		filteredProfiles := make([]api.DataspaceProfile, 0)
		for _, profile := range dProfiles {
			if profileIDMap[profile.ID] {
				filteredProfiles = append(filteredProfiles, profile)
			}
		}
		dProfiles = filteredProfiles
	}
	if len(dProfiles) == 0 {
		return nil, fmt.Errorf("no dataspace profiles found")
	}
	return dProfiles, nil
}

func (p participantService) DisposeProfile(ctx context.Context, tenantID string, participantID string) error {
	return p.trxContext.Execute(ctx, func(c context.Context) error {
		profile, err := p.participantStore.FindByID(c, participantID)
		if err != nil {
			return err
		}
		if profile.TenantID != tenantID {
			return types.ErrNotFound
		}
		states := make([]string, 0, len(profile.VPAs))
		for _, vpa := range profile.VPAs {
			if vpa.State != api.DeploymentStateActive {
				states = append(states, vpa.ID+":"+vpa.State.String())
			}
		}
		if len(states) > 0 {
			return fmt.Errorf("cannot dispose VPAs %s in states: %s", participantID, strings.Join(states, ","))
		}
		stateData, found := profile.Properties[model.VPAStateData]
		if !found {
			return fmt.Errorf("profile is not deployed or is missing state data: %s", participantID)
		}

		oManifest := model.OrchestrationManifest{
			ID:                uuid.New().String(),
			CorrelationID:     participantID,
			OrchestrationType: model.VPADisposeType,
			Payload:           make(map[string]any),
		}

		oManifest.Payload[model.ParticipantIdentifier] = profile.Identifier
		oManifest.Payload[model.VPAStateData] = stateData

		vpaManifests := make([]model.VPAManifest, 0, len(profile.VPAs))
		for i, vpa := range profile.VPAs {
			vpaManifest := model.VPAManifest{
				ID:             vpa.ID,
				VPAType:        vpa.Type,
				CellID:         vpa.CellID,
				ExternalCellID: vpa.ExternalCellID,
				Properties:     vpa.Properties,
			}
			vpaManifests = append(vpaManifests, vpaManifest)

			// Set to disposing - updates the slice element
			profile.VPAs[i].State = api.DeploymentStateDisposing
		}

		oManifest.Payload[model.VPAData] = vpaManifests

		err = p.participantStore.Update(c, profile)
		if err != nil {
			return fmt.Errorf("error disposing participant %s: %w", participantID, err)
		}

		// Only send the orchestration message if the storage operation succeeded. If the send fails, the transaction
		// will be rolled back.
		err = p.provisionClient.Send(ctx, oManifest)
		if err != nil {
			return fmt.Errorf("error disposing participant %s: %w", participantID, err)
		}

		return nil
	})
}

// executeStoreIterator wraps store iterator operations in a transaction context
func (p participantService) executeStoreIterator(ctx context.Context, storeOp func(context.Context) iter.Seq2[*api.ParticipantProfile, error]) iter.Seq2[*api.ParticipantProfile, error] {
	return func(yield func(*api.ParticipantProfile, error) bool) {
		err := p.trxContext.Execute(ctx, func(ctx context.Context) error {
			for profile, err := range storeOp(ctx) {
				if !yield(profile, err) {
					return context.Canceled
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			yield(&api.ParticipantProfile{}, err)
		}
	}
}

func generateCredentialSpecs(
	participantRoles map[string][]string,
	dProfiles []api.DataspaceProfile) []model.CredentialSpec {

	credentials := make([]model.CredentialSpec, 0)
	for _, profile := range dProfiles {
		for _, credentialSpec := range profile.DataspaceSpec.CredentialSpecs {
			credentialRole := credentialSpec.ParticipantRole
			if credentialRole == "" {
				credentials = append(credentials, credentialSpec)
			} else {
				rolesInDataspace, found := participantRoles[profile.ID]
				if found {
					for _, role := range rolesInDataspace {
						if role == credentialRole {
							credentials = append(credentials, credentialSpec)
							break
						}
					}
				}
			}
		}
	}
	return credentials
}

type vpaCallbackHandler struct {
	participantStore store.EntityStore[*api.ParticipantProfile]
	trxContext       store.TransactionContext
	monitor          system.LogMonitor
}

func (h vpaCallbackHandler) handleDeploy(ctx context.Context, response model.OrchestrationResponse) error {
	profileUpdater := func(profile *api.ParticipantProfile, resp model.OrchestrationResponse) {
		// Place all output values under VPStateData key
		vpaProps := make(map[string]any)
		for key, value := range resp.Properties {
			vpaProps[key] = value
		}
		profile.Properties[model.VPAStateData] = vpaProps

		targetState := api.DeploymentStateActive
		if !resp.Success {
			targetState = api.DeploymentStateError
		}
		for i, vpa := range profile.VPAs {
			vpa.State = targetState
			// TODO update timestamp based on returned data
			profile.VPAs[i] = vpa // Use range index because vpa is a copy
		}
	}
	return h.handle(ctx, response, profileUpdater)
}

func (h vpaCallbackHandler) handleDispose(ctx context.Context, response model.OrchestrationResponse) error {
	return h.handle(ctx, response, func(profile *api.ParticipantProfile, resp model.OrchestrationResponse) {
		for i, vpa := range profile.VPAs {
			// Update state
			vpa.State = api.DeploymentStateDisposed
			// TODO update timestamp based on returned data
			profile.VPAs[i] = vpa // Use range index because vpa is a copy
		}
	})
}

// handle processes the asynchronous response to participant VPA deployment request.
func (h vpaCallbackHandler) handle(
	ctx context.Context,
	response model.OrchestrationResponse,
	handler func(profile *api.ParticipantProfile, resp model.OrchestrationResponse)) error {

	return h.trxContext.Execute(ctx, func(c context.Context) error {
		// Note de-duplication does not need to be performed as this operation is idempotent
		profile, err := h.participantStore.FindByID(c, response.CorrelationID)
		if err != nil {
			h.monitor.Infof("Error retrieving participant profile '%s' for manifest %s: %w", response.CorrelationID, response.ManifestID, err)
			// Do not return error as this is fatal and the message must be acked
			return nil
		}
		switch {
		case response.Success:
			handler(profile, response)
		default:
			// TODO update VPA status
			profile.Error = true
			profile.ErrorDetail = response.ErrorDetail
			handler(profile, response)
		}
		err = h.participantStore.Update(c, profile)
		if err != nil {
			return fmt.Errorf("error updating participant profile %s processing VPA response for manifest %s: %w", response.CorrelationID, response.ManifestID, err)
		}
		return nil
	})
}
