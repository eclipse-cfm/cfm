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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/metaform/connector-fabric-manager/assembly/routing"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
)

type response struct {
	Message string `json:"message"`
}

type HandlerServiceAssembly struct {
	system.DefaultServiceAssembly
}

func (h *HandlerServiceAssembly) Name() string {
	return "Provision Manager Handlers"
}

func (h *HandlerServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{routing.RouterKey, api.ProvisionManagerKey, api.DefinitionStoreKey}
}

func (h *HandlerServiceAssembly) Init(context *system.InitContext) error {
	router := context.Registry.Resolve(routing.RouterKey).(chi.Router)
	router.Use(middleware.Recoverer)

	provisionManager := context.Registry.Resolve(api.ProvisionManagerKey).(api.ProvisionManager)
	definitionManager := context.Registry.Resolve(api.DefinitionManagerKey).(api.DefinitionManager)
	txContext := context.Registry.Resolve(store.TransactionContextKey).(store.TransactionContext)
	handler := NewHandler(provisionManager, definitionManager, txContext, context.LogMonitor)

	router.Route("/api/v1alpha1", func(r chi.Router) {
		h.registerV1Alpha1(r, handler)
	})

	return nil
}

func (h *HandlerServiceAssembly) registerV1Alpha1(router chi.Router, handler *PMHandler) {
	h.registerActivityDefinitionRoutes(router, handler)
	h.registerOrchestrationDefinitionRoutes(router, handler)

	h.registerOrchestrationRoutes(router, handler)
	router.Get("/health", handler.health)
}

func (h *HandlerServiceAssembly) registerOrchestrationRoutes(router chi.Router, handler *PMHandler) {
	router.Route("/orchestrations", func(r chi.Router) {
		r.Post("/", handler.createOrchestration)
		r.Post("/query", func(w http.ResponseWriter, req *http.Request) {
			handler.queryOrchestrations(w, req, "/orchestrations/query")
		})

		r.Route("/{orchestrationID}", func(r chi.Router) {
			r.Post("/", func(w http.ResponseWriter, req *http.Request) {
				id, found := handler.ExtractPathVariable(w, req, "orchestrationID")
				if !found {
					return
				}
				//todo: shouldn't this delete the orchestration?
				handler.deleteOrchestrationDefinition(w, req, id)
			})
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				orchestrationID, found := handler.ExtractPathVariable(w, req, "orchestrationID")
				if !found {
					return
				}
				handler.getOrchestration(w, req, orchestrationID)
			})
		})
	})
}

func (h *HandlerServiceAssembly) registerActivityDefinitionRoutes(router chi.Router, handler *PMHandler) {
	router.Route("/activity-definitions", func(r chi.Router) {
		r.Get("/", handler.getActivityDefinitions)
		r.Post("/", handler.createActivityDefinition)
		r.Route("/{activityType}", func(r chi.Router) {
			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				definitionType, found := handler.ExtractPathVariable(w, req, "activityType")
				if !found {
					return
				}
				handler.deleteActivityDefinition(w, req, definitionType)
			})
		})
	})
}

func (h *HandlerServiceAssembly) registerOrchestrationDefinitionRoutes(router chi.Router, handler *PMHandler) {
	router.Route("/orchestration-definitions", func(r chi.Router) {
		r.Get("/", handler.getOrchestrationDefinitions)
		r.Post("/", handler.createOrchestrationDefinition)
		r.Route("/{templateRef}", func(r chi.Router) { // delete-by-template-id
			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				templateRef, found := handler.ExtractPathVariable(w, req, "templateRef")
				if !found {
					return
				}
				handler.deleteOrchestrationDefinition(w, req, templateRef)
			})
			r.Get("/", func(w http.ResponseWriter, request *http.Request) {
				templateRef, found := handler.ExtractPathVariable(w, request, "templateRef")
				if !found {
					return
				}
				handler.getOrchestrationDefinitionsByTemplate(w, request, templateRef)
			})
		})
	})
}
