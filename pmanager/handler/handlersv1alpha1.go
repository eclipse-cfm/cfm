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
	"context"
	"fmt"
	"iter"
	"net/http"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	. "github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/handler"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/model/v1alpha1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PMHandler struct {
	handler.HttpHandler
	provisionManager  api.ProvisionManager
	definitionManager api.DefinitionManager
	txContext         store.TransactionContext
}

func NewHandler(provisionManager api.ProvisionManager, definitionManager api.DefinitionManager, txContext store.TransactionContext, monitor system.LogMonitor) *PMHandler {
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
	namespace, ok := h.payloadNamespace(w, req, definition.Namespace)
	if !ok {
		return
	}
	definition.Namespace = namespace

	_, err := h.definitionManager.CreateActivityDefinition(req.Context(), v1alpha1.ToActivityDefinition(&definition))
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.Created(w)
}

func (h *PMHandler) createOrchestrationDefinition(w http.ResponseWriter, req *http.Request) {
	_, span := otel.GetTracerProvider().Tracer("cfm.pmanager.handler").Start(req.Context(), "createOrchestrationDefinition")
	defer span.End()
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var orchestrationTemplate v1alpha1.OrchestrationTemplate
	if !h.ReadPayload(w, req, &orchestrationTemplate) {
		return
	}
	namespace, ok := h.payloadNamespace(w, req, orchestrationTemplate.Namespace)
	if !ok {
		return
	}
	orchestrationTemplate.Namespace = namespace

	span.SetAttributes(attribute.String("orchestration.template_id", orchestrationTemplate.ID), attribute.String("orchestration.namespace", namespace))
	span.AddEvent("Payload read successfully")
	if err := orchestrationTemplate.ValidateCompensation(); err != nil {
		span.RecordError(err)
		h.HandleError(w, err)
		return
	}
	if len(orchestrationTemplate.Compensation) == 0 {
		span.RecordError(fmt.Errorf("no compensation entry found"))
		h.Monitor.Warnf("Orchestration template [%s] does not contain a compensation entry. Failed orchestrations of this template will not be compensated automatically", orchestrationTemplate.ID)
	}

	templateRef, definitions := v1alpha1.ToOrchestrationDefinition(&orchestrationTemplate)
	for _, def := range definitions {
		_, err := h.definitionManager.CreateOrchestrationDefinition(req.Context(), def)
		if err != nil {
			h.HandleError(w, err)
			span.RecordError(err)
			return
		}
		span.AddEvent("Orchestration definition created", trace.WithAttributes(attribute.String("orchestration.definition_id", def.GetID())))
	}

	h.ResponseCreated(w, v1alpha1.IDResponse{ID: templateRef, Description: "ID of the Orchestration Template"})
}

func (h *PMHandler) createOrchestration(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	var manifest model.OrchestrationManifest
	if !h.ReadPayload(w, req, &manifest) {
		return
	}
	namespace, ok := h.payloadNamespace(w, req, manifest.Namespace)
	if !ok {
		return
	}
	manifest.Namespace = namespace

	orchestration, err := h.provisionManager.Start(req.Context(), &manifest)
	if err != nil {
		h.HandleError(w, err)
		return
	}
	h.ResponseAccepted(w, h.toOrchestrationResponse(req, orchestration))
}

func (h *PMHandler) health(w http.ResponseWriter, _ *http.Request) {
	response := response{Message: "OK"}
	h.ResponseOK(w, response)
}

func (h *PMHandler) deleteOrchestrationDefinition(w http.ResponseWriter, req *http.Request, templateRef string) {
	if h.InvalidMethod(w, req, http.MethodDelete) {
		return
	}

	namespace := requestNamespace(req)
	err := h.definitionManager.DeleteOrchestrationDefinition(req.Context(), namespace, templateRef)
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

	namespace := requestNamespace(req)
	err := h.definitionManager.DeleteActivityDefinition(req.Context(), namespace, api.ActivityType(aType))
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.OK(w)
}

func (h *PMHandler) queryOrchestrations(w http.ResponseWriter, req *http.Request, path string) {
	namespace := requestNamespace(req)
	handler.QueryEntities[*api.OrchestrationEntry](
		&h.HttpHandler,
		w,
		req,
		path,
		func(ctx context.Context, predicate query.Predicate) (int64, error) {
			return h.provisionManager.CountOrchestrations(ctx, namespace, predicate)
		},
		func(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
			return h.provisionManager.QueryOrchestrations(ctx, namespace, predicate, options)
		},
		func(entry *api.OrchestrationEntry) any {
			return v1alpha1.ToOrchestrationEntry(entry)
		},
		h.txContext)
}

func (h *PMHandler) getOrchestration(w http.ResponseWriter, req *http.Request, id string) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}
	namespace := requestNamespace(req)
	orchestration, err := h.provisionManager.GetOrchestration(req.Context(), namespace, id)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	h.ResponseOK(w, h.toOrchestrationResponse(req, orchestration))
}

// toOrchestrationResponse converts an orchestration to its API representation. Its processing and output data carry
// the caller's payload and the values activities produced, so they are only returned to callers with the admin scope.
func (h *PMHandler) toOrchestrationResponse(req *http.Request, orchestration *api.Orchestration) v1alpha1.Orchestration {
	response := v1alpha1.ToOrchestration(orchestration)
	if !canSeeOrchestrationData(req) {
		response.ProcessingData = map[string]any{}
		response.OutputData = map[string]any{}
	}
	return response
}

func canSeeOrchestrationData(req *http.Request) bool {
	// When authentication is disabled, the anonymous claims grant the admin scope.
	claims, ok := cfmauth.ClaimsFromContext(req.Context())
	return ok && claims != nil && hasPermission(claims, requestNamespace(req), permissionAdmin)
}

func (h *PMHandler) getActivityDefinitions(w http.ResponseWriter, req *http.Request) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}
	namespace := requestNamespace(req)
	definitions, err := h.definitionManager.GetActivityDefinitions(req.Context(), namespace)
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
	namespace := requestNamespace(req)
	definitions, err := h.definitionManager.GetOrchestrationDefinitions(req.Context(), namespace)
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

func (h *PMHandler) getOrchestrationDefinitionsByTemplate(w http.ResponseWriter, req *http.Request, templateRef string) {
	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}

	namespace := requestNamespace(req)
	defs, err := h.definitionManager.GetOrchestrationDefinitionsByTemplate(req.Context(), namespace, templateRef)
	if err != nil {
		h.HandleError(w, err)
		return
	}

	dtos := Collect(Map(From(defs), func(def api.OrchestrationDefinition) v1alpha1.OrchestrationDefinitionDto {
		return *v1alpha1.ToOrchestrationDefinitionDto(&def)
	}))

	h.ResponseOK(w, dtos)
}
