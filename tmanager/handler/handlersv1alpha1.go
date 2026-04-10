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

package handler

import (
	"net/http"

	"github.com/eclipse-cfm/cfm/common/handler"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/eclipse-cfm/cfm/tmanager/model/v1alpha1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type TMHandler struct {
	handler.HttpHandler
	tenantService      api.TenantService
	participantService api.ParticipantProfileService
	cellService        api.CellService
	dataspaceService   api.DataspaceProfileService
	txContext          store.TransactionContext
}

func NewHandler(tenantService api.TenantService, participantService api.ParticipantProfileService, cellService api.CellService, dataspaceService api.DataspaceProfileService, txContext store.TransactionContext, monitor system.LogMonitor) *TMHandler {
	return &TMHandler{
		HttpHandler: handler.HttpHandler{
			Monitor: monitor,
		},
		tenantService:      tenantService,
		participantService: participantService,
		cellService:        cellService,
		dataspaceService:   dataspaceService,
		txContext:          txContext,
	}
}

func (h *TMHandler) getParticipantProfile(
	w http.ResponseWriter,
	req *http.Request,
	tenantID string,
	participantID string) {

	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}

	profile, err := h.participantService.GetProfile(req.Context(), tenantID, participantID)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	response := v1alpha1.ToParticipantProfile(profile)
	h.ResponseOK(w, response)
}

func (h *TMHandler) getProfilesForTenant(w http.ResponseWriter, req *http.Request, tenantID string) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}
	iter := h.participantService.QueryProfiles(req.Context(), query.Eq("tenantId", tenantID), store.DefaultPaginationOptions())
	results := make([]v1alpha1.ParticipantProfile, 0)
	for profile, err := range iter {
		if err != nil {
			h.HandleError(w, err)
		}
		transformed := v1alpha1.ToParticipantProfile(profile)
		results = append(results, *transformed)
	}
	h.ResponseOK(w, results)
}

func (h *TMHandler) deployParticipantProfile(
	w http.ResponseWriter,
	req *http.Request,
	tenantID string) {

	_, span := otel.GetTracerProvider().Tracer("cfm.tmanager.handler").Start(req.Context(), "deployParticipantProfile")
	defer span.End()

	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var newDeployment v1alpha1.NewParticipantProfileDeployment
	if !h.ReadPayload(w, req, &newDeployment) {
		return
	}

	span.SetAttributes(attribute.String("tenant.id", tenantID),
		attribute.String("profile.did", newDeployment.Identifier),
		attribute.String("cell.id", newDeployment.CellID))
	converted := v1alpha1.ToAPINewParticipantProfileDeployment(&newDeployment)
	span.AddEvent("Converted participant deployment")
	// TODO support specific cell selection
	profile, err := h.participantService.DeployProfile(
		req.Context(),
		tenantID,
		converted)

	if err != nil {
		span.RecordError(err)
		h.HandleError(w, err)
		return
	}

	span.AddEvent("Profile deployed successfully", trace.WithAttributes(attribute.String("profile.id", profile.ID)))
	response := v1alpha1.ToParticipantProfile(profile)
	h.ResponseAccepted(w, response)
}

func (h *TMHandler) disposeParticipantProfile(
	w http.ResponseWriter,
	req *http.Request,
	tenantID string,
	participantID string) {

	if h.InvalidMethod(w, req, http.MethodDelete) {
		return
	}

	err := h.participantService.DisposeProfile(req.Context(), tenantID, participantID)
	if err != nil {
		h.HandleError(w, err)
	}

	h.Accepted(w)
}

func (h *TMHandler) queryParticipantProfiles(w http.ResponseWriter, req *http.Request, path string) {
	handler.QueryEntities[*api.ParticipantProfile](
		&h.HttpHandler,
		w,
		req,
		path,
		h.participantService.QueryProfilesCount,
		h.participantService.QueryProfiles,
		func(profile *api.ParticipantProfile) any {
			return v1alpha1.ToParticipantProfile(profile)
		},
		h.txContext)
}

func (h *TMHandler) createTenant(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var newTenant v1alpha1.NewTenant
	if !h.ReadPayload(w, req, &newTenant) {
		return
	}

	tenant, err := h.tenantService.CreateTenant(req.Context(), v1alpha1.NewAPITenant(&newTenant))
	if err != nil {
		h.HandleError(w, err)
		return
	}

	response := v1alpha1.ToTenant(tenant)
	h.ResponseCreated(w, response)
}

func (h *TMHandler) patchTenant(w http.ResponseWriter, req *http.Request, tenantID string) {
	if h.InvalidMethod(w, req, http.MethodPatch) {
		return
	}
	var diff v1alpha1.TenantPropertiesDiff
	if !h.ReadPayload(w, req, &diff) {
		return
	}

	err := h.tenantService.PatchTenant(req.Context(), tenantID, diff.Properties, diff.Removed)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.OK(w)
}

func (h *TMHandler) deleteTenant(w http.ResponseWriter, req *http.Request, tenantID string) {
	if h.InvalidMethod(w, req, http.MethodDelete) {
		return
	}
	err := h.tenantService.DeleteTenant(req.Context(), tenantID)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.OK(w)
}

func (h *TMHandler) getTenants(w http.ResponseWriter, req *http.Request, path string) {
	handler.ListEntities[*api.Tenant](
		&h.HttpHandler,
		w,
		req,
		path,
		h.tenantService.GetTenantsCount,
		h.tenantService.GetTenants,
		func(tenant *api.Tenant) any {
			return v1alpha1.ToTenant(tenant)
		},
		h.txContext)
}

func (h *TMHandler) getTenant(w http.ResponseWriter, req *http.Request, tenantID string) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}

	tenant, err := h.tenantService.GetTenant(req.Context(), tenantID)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	response := v1alpha1.ToTenant(tenant)
	h.ResponseOK(w, response)
}

func (h *TMHandler) queryTenants(w http.ResponseWriter, req *http.Request, path string) {
	handler.QueryEntities[*api.Tenant](
		&h.HttpHandler,
		w,
		req,
		path,
		h.tenantService.QueryTenantsCount,
		h.tenantService.QueryTenants,
		func(tenant *api.Tenant) any {
			return v1alpha1.ToTenant(tenant)
		},
		h.txContext)
}

func (h *TMHandler) createCell(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var newCell v1alpha1.NewCell
	if !h.ReadPayload(w, req, &newCell) {
		return
	}

	cell := v1alpha1.NewAPICell(&newCell)

	recordedCell, err := h.cellService.RecordExternalDeployment(req.Context(), cell)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.ResponseCreated(w, v1alpha1.ToCell(recordedCell))
}

func (h *TMHandler) deleteCell(w http.ResponseWriter, req *http.Request, cellID string) {
	if h.InvalidMethod(w, req, http.MethodDelete) {
		return
	}

	err := h.cellService.DeleteCell(req.Context(), cellID)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.OK(w)
}

func (h *TMHandler) getCells(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}

	cells, err := h.cellService.ListCells(req.Context())
	if err != nil {
		h.HandleError(w, err)
		return
	}

	converted := make([]v1alpha1.Cell, len(cells))
	for i, cell := range cells {
		converted[i] = *v1alpha1.ToCell(&cell)
	}
	h.ResponseOK(w, converted)
}

func (h *TMHandler) createDataspaceProfile(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var newProfile v1alpha1.NewDataspaceProfile
	if !h.ReadPayload(w, req, &newProfile) {
		return
	}

	dProfile := v1alpha1.NewAPIDataspaceProfile(&newProfile)
	result, err := h.dataspaceService.CreateProfile(req.Context(), dProfile)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	response := v1alpha1.ToDataspaceProfile(result)
	h.ResponseCreated(w, response)
}

func (h *TMHandler) deleteDataspaceProfile(w http.ResponseWriter, req *http.Request, profileID string) {
	if h.InvalidMethod(w, req, http.MethodDelete) {
		return
	}

	err := h.dataspaceService.DeleteProfile(req.Context(), profileID)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.OK(w)
}

func (h *TMHandler) getDataspaceProfiles(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}

	profiles, err := h.dataspaceService.ListProfiles(req.Context())
	if err != nil {
		h.HandleError(w, err)
		return
	}

	converted := make([]v1alpha1.DataspaceProfile, len(profiles))
	for i, profile := range profiles {
		converted[i] = *v1alpha1.ToDataspaceProfile(&profile)
	}
	h.ResponseOK(w, converted)
}

func (h *TMHandler) getDataspaceProfile(w http.ResponseWriter, req *http.Request, id string) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}

	profile, err := h.dataspaceService.GetProfile(req.Context(), id)
	if err != nil {
		h.HandleError(w, err)
		return
	}
	h.ResponseOK(w, v1alpha1.ToDataspaceProfile(profile))
}

func (h *TMHandler) deployDataspaceProfile(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var newDeployment v1alpha1.NewDataspaceProfileDeployment
	if !h.ReadPayload(w, req, &newDeployment) {
		return
	}

	err := h.dataspaceService.DeployProfile(req.Context(), newDeployment.ProfileID, newDeployment.CellID)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.Accepted(w)
}

func (h *TMHandler) health(w http.ResponseWriter, _ *http.Request) {
	h.ResponseOK(w, response{Message: "OK"})
}
