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

	cfmauth "github.com/eclipse-cfm/cfm/assembly/auth"
	"github.com/eclipse-cfm/cfm/assembly/routing"
	cfmhandler "github.com/eclipse-cfm/cfm/common/handler"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	scopePmRead  = "provision-manager-api:read"
	scopePmWrite = "provision-manager-api:write"
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
	return []system.ServiceType{
		routing.RouterKey,
		api.ProvisionManagerKey,
		api.DefinitionStoreKey,
		cfmauth.ValidatorKey,
	}
}

func (h *HandlerServiceAssembly) Init(context *system.InitContext) error {
	router := context.Registry.Resolve(routing.RouterKey).(chi.Router)
	router.Use(middleware.Recoverer)

	provisionManager := context.Registry.Resolve(api.ProvisionManagerKey).(api.ProvisionManager)
	definitionManager := context.Registry.Resolve(api.DefinitionManagerKey).(api.DefinitionManager)
	txContext := context.Registry.Resolve(store.TransactionContextKey).(store.TransactionContext)
	validator := context.Registry.Resolve(cfmauth.ValidatorKey).(cfmauth.AuthValidator)
	handler := NewHandler(provisionManager, definitionManager, txContext, context.LogMonitor)

	// Health is unauthenticated so it can be used by infrastructure probes without a token.
	router.Get("/health", handler.health)

	router.Route("/api/v1alpha1", func(r chi.Router) {
		r.Use(validator.Middleware())
		h.registerV1Alpha1(r, handler)
	})

	return nil
}

func (h *HandlerServiceAssembly) registerV1Alpha1(router chi.Router, handler *PMHandler) {
	h.registerActivityDefinitionRoutes(router, handler)
	h.registerOrchestrationDefinitionRoutes(router, handler)
	h.registerOrchestrationRoutes(router, handler)
}

func (h *HandlerServiceAssembly) registerOrchestrationRoutes(router chi.Router, handler *PMHandler) {
	router.Route("/orchestrations", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmWrite)) {
				return
			}
			handler.createOrchestration(w, req)
		})
		r.Post("/query", func(w http.ResponseWriter, req *http.Request) {
			if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmRead)) {
				return
			}
			handler.queryOrchestrations(w, req, "/orchestrations/query")
		})

		r.Route("/{orchestrationID}", func(r chi.Router) {
			r.Post("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmWrite)) {
					return
				}
				id, found := handler.ExtractPathVariable(w, req, "orchestrationID")
				if !found {
					return
				}
				//todo: shouldn't this delete the orchestration?
				handler.deleteOrchestrationDefinition(w, req, id)
			})
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmRead)) {
					return
				}
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
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmRead)) {
				return
			}
			handler.getActivityDefinitions(w, req)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmWrite)) {
				return
			}
			handler.createActivityDefinition(w, req)
		})
		r.Route("/{activityType}", func(r chi.Router) {
			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmWrite)) {
					return
				}
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
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmRead)) {
				return
			}
			handler.getOrchestrationDefinitions(w, req)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmWrite)) {
				return
			}
			handler.createOrchestrationDefinition(w, req)
		})
		r.Route("/{templateRef}", func(r chi.Router) {
			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.IsAuthorized(w, req, cfmhandler.RequireScope(scopePmWrite)) {
					return
				}
				templateRef, found := handler.ExtractPathVariable(w, req, "templateRef")
				if !found {
					return
				}
				handler.deleteOrchestrationDefinition(w, req, templateRef)
			})
			r.Get("/", func(w http.ResponseWriter, request *http.Request) {
				if !handler.IsAuthorized(w, request, cfmhandler.RequireScope(scopePmRead)) {
					return
				}
				templateRef, found := handler.ExtractPathVariable(w, request, "templateRef")
				if !found {
					return
				}
				handler.getOrchestrationDefinitionsByTemplate(w, request, templateRef)
			})
		})
	})
}
