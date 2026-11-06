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
	"net/http"
	"net/http/httptest"
	"testing"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestOrchestrationRoutes_NoPostOnOrchestrationID(t *testing.T) {
	router := chi.NewRouter()
	assembly := &HandlerServiceAssembly{}
	assembly.registerOrchestrationRoutes(router, NewHandler(nil, nil, nil, system.NoopMonitor{}))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/orchestrations/template-1", nil))

	// The route used to dispatch to the orchestration-definition delete handler. It must not be routed at all now:
	// the router itself rejects the method, so no handler writes a response body.
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Empty(t, w.Body.String())
}

// newClaimsRouter registers the PM routes behind a middleware that attaches the given claims, standing in for the
// auth validator.
func newClaimsRouter(handler *PMHandler, claims *cfmauth.Claims) chi.Router {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cfmauth.ContextKey{}, claims)))
		})
	})
	(&HandlerServiceAssembly{}).registerV1Alpha1(router, handler)
	return router
}

func TestDefinitionRoutes_RequireAdminScope(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/activity-definitions"},
		{http.MethodDelete, "/activity-definitions/type-1"},
		{http.MethodPost, "/orchestration-definitions"},
		{http.MethodDelete, "/orchestration-definitions/template-1"},
	}
	writer := &cfmauth.Claims{Subject: "client-1", Scopes: []string{scopePmRead, scopePmWrite}}
	router := newClaimsRouter(NewHandler(nil, nil, nil, system.NoopMonitor{}), writer)

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), scopePmAdmin)
		})
	}
}
