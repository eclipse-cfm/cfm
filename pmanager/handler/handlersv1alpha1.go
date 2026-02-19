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

	"github.com/metaform/connector-fabric-manager/common/handler"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/metaform/connector-fabric-manager/pmanager/model/v1alpha1"
)

type PMHandler struct {
	handler.HttpHandler
	provisionManager  api.ProvisionManager
	definitionManager api.DefinitionManager
	txContext         store.TransactionContext
}

func NewHandler(
	provisionManager api.ProvisionManager,
	definitionManager api.DefinitionManager,
	txContext store.TransactionContext,
	monitor system.LogMonitor) *PMHandler {
	return &PMHandler{
		HttpHandler: handler.HttpHandler{
			Monitor: monitor,
		},
		provisionManager:  provisionManager,
		definitionManager: definitionManager,
		txContext:         txContext,
	}
}

func (h *PMHandler) createActivityDefinition(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var definition v1alpha1.ActivityDefinitionDto
	if !h.ReadPayload(w, req, &definition) {
		return
	}

	_, err := h.definitionManager.CreateActivityDefinition(req.Context(), v1alpha1.ToActivityDefinition(&definition))
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.Created(w)
}

func (h *PMHandler) createOrchestrationDefinition(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var orchestrationTemplate v1alpha1.OrchestrationTemplate
	if !h.ReadPayload(w, req, &orchestrationTemplate) {
		return
	}

	definitions := v1alpha1.ToOrchestrationDefinition(&orchestrationTemplate)
	for _, def := range definitions {
		_, err := h.definitionManager.CreateOrchestrationDefinition(req.Context(), def)
		if err != nil {
			h.HandleError(w, err)
			return
		}
	}

	h.Created(w)
}

func (h *PMHandler) createOrchestration(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var manifest model.OrchestrationManifest
	if !h.ReadPayload(w, req, &manifest) {
		return
	}

	orchestration, err := h.provisionManager.Start(req.Context(), &manifest)
	if err != nil {
		h.HandleError(w, err)
		return
	}
	h.ResponseAccepted(w, orchestration)
}

func (h *PMHandler) health(w http.ResponseWriter, _ *http.Request) {
	response := response{Message: "OK"}
	h.ResponseOK(w, response)
}

func (h *PMHandler) deleteOrchestrationDefinition(w http.ResponseWriter, req *http.Request, oType string) {
	if h.InvalidMethod(w, req, http.MethodDelete) {
		return
	}

	err := h.definitionManager.DeleteOrchestrationDefinition(req.Context(), model.OrchestrationType(oType))
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.OK(w)
}

func (h *PMHandler) deleteActivityDefinition(w http.ResponseWriter, req *http.Request, aType string) {
	if h.InvalidMethod(w, req, http.MethodDelete) {
		return
	}

	err := h.definitionManager.DeleteActivityDefinition(req.Context(), api.ActivityType(aType))
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.OK(w)
}

func (h *PMHandler) queryOrchestrations(w http.ResponseWriter, req *http.Request, path string) {
	handler.QueryEntities[*api.OrchestrationEntry](
		&h.HttpHandler,
		w,
		req,
		path,
		h.provisionManager.CountOrchestrations,
		h.provisionManager.QueryOrchestrations,
		func(entry *api.OrchestrationEntry) any {
			return v1alpha1.ToOrchestrationEntry(entry)
		},
		h.txContext)
}

func (h *PMHandler) getOrchestration(w http.ResponseWriter, req *http.Request, id string) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}
	orchestration, err := h.provisionManager.GetOrchestration(req.Context(), id)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	response := v1alpha1.ToOrchestration(orchestration)
	h.ResponseOK(w, response)
}

func (h *PMHandler) getActivityDefinitions(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}
	definitions, err := h.definitionManager.GetActivityDefinitions(req.Context())
	if err != nil {
		h.HandleError(w, err)
		return
	}
	converted := make([]v1alpha1.ActivityDefinitionDto, len(definitions))
	for i, def := range definitions {
		converted[i] = *v1alpha1.ToActivityDefinitionDto(&def)
	}

	h.ResponseOK(w, converted)
}

func (h *PMHandler) getOrchestrationDefinitions(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}
	definitions, err := h.definitionManager.GetOrchestrationDefinitions(req.Context())
	if err != nil {
		h.HandleError(w, err)
		return
	}
	converted := make([]v1alpha1.OrchestrationDefinitionDto, len(definitions))
	for i, def := range definitions {
		converted[i] = *v1alpha1.ToOrchestrationDefinitionDto(&def)
	}

	h.ResponseOK(w, converted)
}
