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
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	scopeTmRead  = "tenant-manager-api:read"
	scopeTmWrite = "tenant-manager-api:write"
)

type response struct {
	Message string `json:"message"`
}

type HandlerServiceAssembly struct {
	system.DefaultServiceAssembly
}

func (h *HandlerServiceAssembly) Name() string {
	return "Tenant Manager Handlers"
}

func (h *HandlerServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{}
}

func (h *HandlerServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{
		api.ParticipantProfileServiceKey,
		api.CellServiceKey,
		api.DataspaceProfileServiceKey,
		routing.RouterKey,
		cfmauth.ValidatorKey,
	}
}

func (h *HandlerServiceAssembly) Init(context *system.InitContext) error {
	router := context.Registry.Resolve(routing.RouterKey).(chi.Router)
	router.Use(middleware.Recoverer)

	tenantService := context.Registry.Resolve(api.TenantServiceKey).(api.TenantService)
	participantService := context.Registry.Resolve(api.ParticipantProfileServiceKey).(api.ParticipantProfileService)
	cellService := context.Registry.Resolve(api.CellServiceKey).(api.CellService)
	dataspaceService := context.Registry.Resolve(api.DataspaceProfileServiceKey).(api.DataspaceProfileService)
	txContext := context.Registry.Resolve(store.TransactionContextKey).(store.TransactionContext)
	validator := context.Registry.Resolve(cfmauth.ValidatorKey).(cfmauth.AuthValidator)

	handler := NewHandler(tenantService, participantService, cellService, dataspaceService, txContext, context.LogMonitor)

	router.Route("/api/v1alpha1", func(r chi.Router) {
		r.Use(validator.Middleware())
		h.registerV1Alpha1(r, handler)
	})

	return nil
}

func (h *HandlerServiceAssembly) registerV1Alpha1(router chi.Router, handler *TMHandler) {
	h.registerTenantRoutes(router, handler)
	h.registerProfileQueryRoutes(router, handler)
	h.registerCellRoutes(router, handler)
	h.registerDataspaceProfileRoutes(router, handler)
}

func (h *HandlerServiceAssembly) registerProfileQueryRoutes(router chi.Router, handler *TMHandler) {
	router.Route("/participant-profiles", func(r chi.Router) {
		r.Post("/query", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmRead) {
				return
			}
			handler.queryParticipantProfiles(w, req, "/participant-profiles/query")
		})
	})
}

func (h *HandlerServiceAssembly) registerCellRoutes(router chi.Router, handler *TMHandler) {
	router.Route("/cells", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmRead) {
				return
			}
			handler.getCells(w, req)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmWrite) {
				return
			}
			handler.createCell(w, req)
		})
		r.Route("/{cellID}", func(r chi.Router) {
			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.HasRequiredScope(w, req, scopeTmWrite) {
					return
				}
				cellID, found := handler.ExtractPathVariable(w, req, "cellID")
				if !found {
					return
				}
				handler.deleteCell(w, req, cellID)
			})
		})
	})
}

func (h *HandlerServiceAssembly) registerDataspaceProfileRoutes(router chi.Router, handler *TMHandler) {
	router.Route("/dataspace-profiles", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmRead) {
				return
			}
			handler.getDataspaceProfiles(w, req)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmWrite) {
				return
			}
			handler.createDataspaceProfile(w, req)
		})
		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmRead) {
				return
			}
			id, found := handler.ExtractPathVariable(w, req, "id")
			if !found {
				return
			}
			handler.getDataspaceProfile(w, req, id)
		})
		r.Delete("/{profileID}", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmWrite) {
				return
			}
			profileID, found := handler.ExtractPathVariable(w, req, "profileID")
			if !found {
				return
			}
			handler.deleteDataspaceProfile(w, req, profileID)
		})
		r.Route("/{id}/deployments", func(r chi.Router) {
			r.Post("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.HasRequiredScope(w, req, scopeTmWrite) {
					return
				}
				handler.deployDataspaceProfile(w, req)
			})
		})
	})
}

func (h *HandlerServiceAssembly) registerTenantRoutes(router chi.Router, handler *TMHandler) {
	router.Route("/tenants", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmRead) {
				return
			}
			handler.getTenants(w, req, "/tenants")
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmWrite) {
				return
			}
			handler.createTenant(w, req)
		})
		r.Post("/query", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmRead) {
				return
			}
			handler.queryTenants(w, req, "/tenants/query")
		})

		r.Route("/{tenantID}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.HasRequiredScope(w, req, scopeTmRead) {
					return
				}
				tenantID, found := handler.ExtractPathVariable(w, req, "tenantID")
				if !found {
					return
				}
				handler.getTenant(w, req, tenantID)
			})
			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.HasRequiredScope(w, req, scopeTmWrite) {
					return
				}
				tenantID, found := handler.ExtractPathVariable(w, req, "tenantID")
				if !found {
					return
				}
				handler.deleteTenant(w, req, tenantID)
			})
			r.Patch("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.HasRequiredScope(w, req, scopeTmWrite) {
					return
				}
				tenantID, found := handler.ExtractPathVariable(w, req, "tenantID")
				if !found {
					return
				}
				handler.patchTenant(w, req, tenantID)
			})
			h.registerParticipantRoutes(r, handler)
		})
	})
}

func (h *HandlerServiceAssembly) registerParticipantRoutes(r chi.Router, handler *TMHandler) {
	r.Route("/participant-profiles", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmRead) {
				return
			}
			tenantID, found := handler.ExtractPathVariable(w, req, "tenantID")
			if !found {
				return
			}
			handler.getProfilesForTenant(w, req, tenantID)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if !handler.HasRequiredScope(w, req, scopeTmWrite) {
				return
			}
			tenantID, found := handler.ExtractPathVariable(w, req, "tenantID")
			if !found {
				return
			}
			handler.deployParticipantProfile(w, req, tenantID)
		})

		r.Route("/{participantID}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.HasRequiredScope(w, req, scopeTmRead) {
					return
				}
				tenantID, found := handler.ExtractPathVariable(w, req, "tenantID")
				if !found {
					return
				}
				participantID, found := handler.ExtractPathVariable(w, req, "participantID")
				if !found {
					return
				}
				handler.getParticipantProfile(w, req, tenantID, participantID)
			})
			r.Delete("/", func(w http.ResponseWriter, req *http.Request) {
				if !handler.HasRequiredScope(w, req, scopeTmWrite) {
					return
				}
				tenantID, found := handler.ExtractPathVariable(w, req, "tenantID")
				if !found {
					return
				}
				participantID, found := handler.ExtractPathVariable(w, req, "participantID")
				if !found {
					return
				}
				handler.disposeParticipantProfile(w, req, tenantID, participantID)
			})
		})
	})
}
