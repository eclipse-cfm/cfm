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

// AnonymousSubject is the subject of the claims that are attached to every request when authentication is disabled.
const AnonymousSubject = "anonymous"

// Claims holds the verified identity and permissions extracted from a bearer token.
type Claims struct {
	Subject string
	Scopes  []string // parsed from the standard space-separated "scope" claim

	// allScopes grants every scope. It is only set by AnonymousClaims, so it cannot be enabled by accident.
	allScopes bool
}

// AnonymousClaims returns the claims attached to requests when authentication is explicitly disabled. They satisfy
// every scope requirement. Authorization checks treat a request without any claims as unauthenticated, so disabling
// authentication must go through these claims rather than by leaving the context empty.
func AnonymousClaims() *Claims {
	return &Claims{Subject: AnonymousSubject, allScopes: true}
}

// HasScope returns true when the claims contain the given scope.
func (c *Claims) HasScope(requiredScope string) bool {
	return c.allScopes || slices.Contains(c.Scopes, requiredScope)
}

// ClaimsFromContext retrieves Claims stored in the context by the auth middleware.
// Returns false when no claims were stored, i.e. the request did not pass through an auth middleware.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(ContextKey{}).(*Claims)
	return c, ok
}
