//  Copyright (c) 2026 Metaform Systems, Inc
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
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

type fakeCellService struct {
	api.CellService
	deletedCellID string
}

func (f *fakeCellService) DeleteCell(_ context.Context, cellID string) error {
	f.deletedCellID = cellID
	return nil
}

// newClaimsRouter registers the TM routes behind a middleware that attaches the given claims, standing in for the
// auth validator.
func newClaimsRouter(handler *TMHandler, claims *cfmauth.Claims) chi.Router {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cfmauth.ContextKey{}, claims)))
		})
	})
	(&HandlerServiceAssembly{}).registerV1Alpha1(router, handler)
	return router
}

func TestPlatformRoutes_RequireAdminScope(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/cells"},
		{http.MethodDelete, "/cells/cell-1"},
		{http.MethodPost, "/dataspace-profiles"},
		{http.MethodDelete, "/dataspace-profiles/profile-1"},
		{http.MethodPost, "/dataspace-profiles/profile-1/deployments"},
	}
	writer := &cfmauth.Claims{Subject: "client-1", Scopes: []string{scopeTmRead, scopeTmWrite}}
	router := newClaimsRouter(newTestHandler(nil, nil), writer)

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), scopeTmAdmin)
		})
	}
}

func TestPlatformRoutes_AllowAdminScope(t *testing.T) {
	cells := &fakeCellService{}
	handler := NewHandler(nil, nil, cells, nil, nil, system.NoopMonitor{})
	admin := &cfmauth.Claims{Subject: "operator", Scopes: []string{scopeTmAdmin}}
	router := newClaimsRouter(handler, admin)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/cells/cell-1", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cell-1", cells.deletedCellID)
}
