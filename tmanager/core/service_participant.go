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

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/google/uuid"
)

type participantService struct {
	participantGenerator participantGenerator
	provisionClient      api.ProvisionClient
	trxContext           store.TransactionContext
	participantStore     store.EntityStore[*api.ParticipantProfile]
	tenantStore          store.EntityStore[*api.Tenant]
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
		if err := checkProfileAccess(ctx, profile); err != nil {
			return nil, err
		}
		return profile, nil
	})
}

func (p participantService) QueryProfilesCount(ctx context.Context, predicate query.Predicate) (int64, error) {
	access, err := api.TenantAccessFromContext(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	err = p.trxContext.Execute(ctx, func(ctx context.Context) error {
		c, err := p.participantStore.CountByPredicate(ctx, access.Restrict(predicate))
		count = c
		return err
	})
	return count, err
}

func (p participantService) QueryProfiles(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.ParticipantProfile, error] {
	access, err := api.TenantAccessFromContext(ctx)
	if err != nil {
		return func(yield func(*api.ParticipantProfile, error) bool) {
			yield(nil, err)
		}
	}
	return p.executeStoreIterator(ctx, func(ctx context.Context) iter.Seq2[*api.ParticipantProfile, error] {
		return p.participantStore.FindByPredicatePaginated(ctx, access.Restrict(predicate), options)
	})
}

func (p participantService) DeployProfile(ctx context.Context, tenantID string, deployment *api.NewParticipantProfileDeployment) (*api.ParticipantProfile, error) {

	// TODO perform property validation against a custom schema
	var oManifest model.OrchestrationManifest
	result, err := store.Trx[api.ParticipantProfile](p.trxContext).AndReturn(ctx, func(ctx context.Context) (*api.ParticipantProfile, error) {
		// The shared lock keeps the tenant from being deleted until the profile is committed, so that a concurrent
		// DeleteTenant either sees the new profile or makes this deployment fail.
		tenant, err := p.tenantStore.FindByIDAndLock(ctx, tenantID, store.LockShared)
		if err != nil {
			return nil, err
		}
		if err := checkTenantAccess(ctx, tenant); err != nil {
			return nil, err
		}

		cells, err := collection.CollectAllDeref(p.cellStore.GetAll(ctx))
		if err != nil {
			return nil, err
		}

		dProfiles, err := p.getFilteredProfiles(ctx, deployment.DataspaceProfileIDs)
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
		participantProfile.Owner = tenant.Owner

		oManifest = model.OrchestrationManifest{
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

		if err := p.lockReferences(ctx, participantProfile); err != nil {
			return nil, err
		}

		setPendingOrchestration(participantProfile, oManifest)
		result, err := p.participantStore.Create(ctx, participantProfile)
		if err != nil {
			return nil, fmt.Errorf("error creating participant %s: %w", deployment.Identifier, err)
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}

	// Only send the orchestration message once the profile is committed. The response can arrive before an open
	// transaction commits, and the callback would then not find the profile and drop the response.
	if err := p.provisionClient.Send(ctx, oManifest); err != nil {
		sendErr := fmt.Errorf("error deploying participant %s: %w", deployment.Identifier, err)
		if err := p.trxContext.Execute(ctx, func(c context.Context) error {
			return p.participantStore.Delete(c, result.ID)
		}); err != nil {
			return nil, errors.Join(sendErr, fmt.Errorf("error removing participant profile %s: %w", result.ID, err))
		}
		return nil, sendErr
	}
	return result, nil
}

// lockReferences share-locks the cells and dataspace profiles the given profile references, so that they can't be deleted
// until the profile is committed. A reference that was deleted after it was read is a conflict.
func (p participantService) lockReferences(ctx context.Context, profile *api.ParticipantProfile) error {
	cellIDs := make(map[string]struct{})
	for _, vpa := range profile.VPAs {
		if vpa.CellID != "" {
			cellIDs[vpa.CellID] = struct{}{}
		}
	}
	for cellID := range cellIDs {
		if _, err := p.cellStore.FindByIDAndLock(ctx, cellID, store.LockShared); err != nil {
			if errors.Is(err, types.ErrNotFound) {
				return types.NewClientError("cell %s was deleted", cellID)
			}
			return err
		}
	}
	for _, profileID := range profile.DataspaceProfileIDs {
		if _, err := p.dataspaceStore.FindByIDAndLock(ctx, profileID, store.LockShared); err != nil {
			if errors.Is(err, types.ErrNotFound) {
				return types.NewClientError("dataspace profile %s was deleted", profileID)
			}
			return err
		}
	}
	return nil
}

// getFilteredProfiles filters dProfiles based on the given dataspace profile IDs. All profiles are returned if the ID list is empty.
func (p participantService) getFilteredProfiles(
	ctx context.Context,
	dataspaceProfileIDs []string) ([]api.DataspaceProfile, error) {

	dProfiles, err := collection.CollectAllDeref(p.dataspaceStore.GetAll(ctx))
	if err != nil {
		return nil, err
	}

	if len(dataspaceProfileIDs) > 0 {
		profileIDMap := make(map[string]bool)
		for _, id := range dataspaceProfileIDs {
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
	var oManifest model.OrchestrationManifest
	err := p.trxContext.Execute(ctx, func(c context.Context) error {
		profile, err := p.participantStore.FindByID(c, participantID)
		if err != nil {
			return err
		}
		if profile.TenantID != tenantID {
			return types.ErrNotFound
		}
		if err := checkProfileAccess(c, profile); err != nil {
			return err
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

		dProfiles, err := p.getFilteredProfiles(c, profile.DataspaceProfileIDs)
		if err != nil {
			return err
		}

		oManifest = model.OrchestrationManifest{
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
		oManifest.Payload[model.CredentialData] = generateCredentialSpecs(profile.ParticipantRoles, dProfiles)

		setPendingOrchestration(profile, oManifest)
		err = p.participantStore.Update(c, profile)
		if err != nil {
			return fmt.Errorf("error disposing participant %s: %w", participantID, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Only send the orchestration message once the profile update is committed, see DeployProfile.
	if err := p.provisionClient.Send(ctx, oManifest); err != nil {
		sendErr := fmt.Errorf("error disposing participant %s: %w", participantID, err)
		if err := p.revertDispose(ctx, participantID, oManifest); err != nil {
			return errors.Join(sendErr, err)
		}
		return sendErr
	}
	return nil
}

// revertDispose returns the profile's VPAs to the active state after the dispose orchestration could not be sent, so
// that the dispose can be retried. The profile is left alone if another orchestration has been started for it since.
func (p participantService) revertDispose(ctx context.Context, participantID string, manifest model.OrchestrationManifest) error {
	return p.trxContext.Execute(ctx, func(c context.Context) error {
		profile, err := p.participantStore.FindByID(c, participantID)
		if err != nil {
			return fmt.Errorf("error reverting dispose of participant %s: %w", participantID, err)
		}
		if !isPendingOrchestration(profile, model.OrchestrationResponse{ManifestID: manifest.ID, OrchestrationType: manifest.OrchestrationType}) {
			return nil
		}
		delete(profile.Properties, pendingOrchestrationKey)
		for i := range profile.VPAs {
			profile.VPAs[i].State = api.DeploymentStateActive
		}
		if err := p.participantStore.Update(c, profile); err != nil {
			return fmt.Errorf("error reverting dispose of participant %s: %w", participantID, err)
		}
		return nil
	})
}

// pendingOrchestrationKey is the profile property recording the orchestration TM last started for the profile. Only a
// response for that orchestration is applied to the profile.
const pendingOrchestrationKey = "cfm.orchestration.pending"

const (
	pendingManifestIDKey = "manifestId"
	pendingTypeKey       = "orchestrationType"
)

// setPendingOrchestration records the manifest as the orchestration the profile is waiting on, replacing any value
// that was supplied with the profile properties.
func setPendingOrchestration(profile *api.ParticipantProfile, manifest model.OrchestrationManifest) {
	if profile.Properties == nil {
		profile.Properties = make(api.Properties)
	}
	profile.Properties[pendingOrchestrationKey] = map[string]any{
		pendingManifestIDKey: manifest.ID,
		pendingTypeKey:       manifest.OrchestrationType.String(),
	}
}

// isPendingOrchestration reports whether the response matches the manifest ID and orchestration type TM recorded when
// it started the profile's current orchestration, or is the dispose orchestration the Provision Manager started to
// compensate it.
func isPendingOrchestration(profile *api.ParticipantProfile, response model.OrchestrationResponse) bool {
	pending, ok := profile.Properties[pendingOrchestrationKey].(map[string]any)
	if !ok {
		return false
	}
	manifestID, _ := pending[pendingManifestIDKey].(string)
	orchestrationType, _ := pending[pendingTypeKey].(string)
	if manifestID == "" {
		return false
	}
	if manifestID == response.ManifestID {
		return orchestrationType == response.OrchestrationType.String()
	}
	return manifestID == response.CompensatedManifestID &&
		orchestrationType == model.VPADeployType.String() &&
		response.OrchestrationType == model.VPADisposeType
}

// checkProfileAccess returns ErrNotFound when the caller may not access the participant profile.
func checkProfileAccess(ctx context.Context, profile *api.ParticipantProfile) error {
	access, err := api.TenantAccessFromContext(ctx)
	if err != nil {
		return err
	}
	if !access.Allows(profile.Owner) {
		return types.ErrNotFound
	}
	return nil
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
		if !isPendingOrchestration(profile, response) {
			// The response does not belong to the orchestration this profile is waiting on: it is a duplicate of an
			// already processed response, or it was forged. Ack it without touching the profile.
			h.monitor.Warnf("Ignoring %s response for participant profile '%s': manifest %s is not the pending orchestration",
				response.OrchestrationType, response.CorrelationID, response.ManifestID)
			return nil
		}
		if response.Success {
			// A failed orchestration stays pending so that the Provision Manager's compensation can still be applied
			delete(profile.Properties, pendingOrchestrationKey)
		}
		switch {
		case response.Success:
			handler(profile, response)
		default:
			profile.Error = true
			// The compensation of a failed deployment must not replace the reason the deployment failed
			if response.CompensatedManifestID == "" || profile.ErrorDetail == "" {
				profile.ErrorDetail = response.ErrorDetail
				profile.ErrorCause = response.ErrorCause
			}
			handler(profile, response)
		}
		err = h.participantStore.Update(c, profile)
		if err != nil {
			return fmt.Errorf("error updating participant profile %s processing VPA response for manifest %s: %w", response.CorrelationID, response.ManifestID, err)
		}
		return nil
	})
}
