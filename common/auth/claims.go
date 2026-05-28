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

package auth

import (
	"context"
	"slices"
)

// ContextKey is the key used to store Claims in a request context.
type ContextKey struct{}

// Claims holds the verified identity and permissions extracted from a bearer token.
type Claims struct {
	Subject string
	Scopes  []string // parsed from the standard space-separated "scope" claim
	Roles   []string // parsed from the IdP-specific roles claim configured via auth.rolesClaim
}

// HasScope returns true when the claims contain the given scope.
func (c *Claims) HasScope(requiredScope string) bool {
	return slices.Contains(c.Scopes, requiredScope)
}

// ClaimsFromContext retrieves Claims stored in the context by the auth middleware.
// Returns false when auth is disabled and no claims were stored.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ContextKey{}).(*Claims)
	return c, ok
}
