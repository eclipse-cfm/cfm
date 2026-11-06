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
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveWithAccess runs the tenant access middleware for a request with the given claims (nil for none) and returns the
// access it attached, if the request reached the next handler.
func serveWithAccess(t *testing.T, claims *cfmauth.Claims) (*httptest.ResponseRecorder, *api.TenantAccess) {
	t.Helper()
	var captured *api.TenantAccess
	next := http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		if access, err := api.TenantAccessFromContext(req.Context()); err == nil {
			captured = &access
		}
	})
	req := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	if claims != nil {
		req = req.WithContext(context.WithValue(req.Context(), cfmauth.ContextKey{}, claims))
	}
	w := httptest.NewRecorder()
	newTestHandler(nil, nil).tenantAccessMiddleware(next).ServeHTTP(w, req)
	return w, captured
}

func TestTenantAccessMiddleware(t *testing.T) {
	t.Run("full access when auth is disabled", func(t *testing.T) {
		_, access := serveWithAccess(t, cfmauth.AnonymousClaims())

		require.NotNil(t, access)
		assert.False(t, access.IsRestricted())
		assert.Equal(t, cfmauth.AnonymousSubject, access.Subject())
	})

	t.Run("no access without claims", func(t *testing.T) {
		w, access := serveWithAccess(t, nil)

		assert.Nil(t, access)
		assert.Equal(t, http.StatusOK, w.Code, "the authorization check, not the middleware, refuses the request")
	})

	t.Run("full access with the admin scope", func(t *testing.T) {
		_, access := serveWithAccess(t, &cfmauth.Claims{Subject: "operator", Scopes: []string{scopeTmWrite, scopeTmAdmin}})

		require.NotNil(t, access)
		assert.False(t, access.IsRestricted())
		assert.Equal(t, "operator", access.Subject())
	})

	t.Run("own tenants only without the admin scope", func(t *testing.T) {
		_, access := serveWithAccess(t, &cfmauth.Claims{Subject: "client-a", Scopes: []string{scopeTmWrite}})

		require.NotNil(t, access)
		assert.True(t, access.IsRestricted())
		assert.True(t, access.Allows("client-a"))
		assert.False(t, access.Allows("client-b"))
		assert.False(t, access.Allows(""))
	})

	t.Run("refuses a token without a subject", func(t *testing.T) {
		w, access := serveWithAccess(t, &cfmauth.Claims{Scopes: []string{scopeTmWrite}})

		assert.Nil(t, access)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
