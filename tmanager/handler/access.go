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
	"net/http"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

// tenantAccessMiddleware attaches the caller's tenant access to the request context. The services use it to limit
// callers without the admin scope to the tenants they own.
func (h *TMHandler) tenantAccessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		claims, ok := cfmauth.ClaimsFromContext(req.Context())
		if !ok || claims == nil {
			// The request is unauthenticated, so the authorization check refuses it. Without an access in the context,
			// the services refuse it as well.
			next.ServeHTTP(w, req)
			return
		}
		var access api.TenantAccess
		switch {
		case claims.HasScope(scopeTmAdmin):
			// This includes the anonymous claims attached when authentication is disabled.
			access = api.AllTenantsAccess(claims.Subject)
		case claims.Subject == "":
			// Without a subject the caller cannot own tenants, and an empty owner would match tenants that have none.
			h.WriteError(w, "Access not granted: token has no subject", http.StatusForbidden)
			return
		default:
			access = api.OwnedTenantsAccess(claims.Subject)
		}
		next.ServeHTTP(w, req.WithContext(api.ContextWithTenantAccess(req.Context(), access)))
	})
}
