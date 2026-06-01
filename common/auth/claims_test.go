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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasScope(t *testing.T) {
	t.Run("returns true when scope is present", func(t *testing.T) {
		c := &Claims{Scopes: []string{"read", "write"}}
		assert.True(t, c.HasScope("read"))
		assert.True(t, c.HasScope("write"))
	})

	t.Run("returns false when scope is absent", func(t *testing.T) {
		c := &Claims{Scopes: []string{"read"}}
		assert.False(t, c.HasScope("admin"))
	})

	t.Run("returns false for empty scopes", func(t *testing.T) {
		c := &Claims{}
		assert.False(t, c.HasScope("read"))
	})
}

func TestHasRole(t *testing.T) {
	t.Run("returns true when role is present", func(t *testing.T) {
		c := &Claims{Roles: []string{"admin", "viewer"}}
		assert.True(t, c.HasRole("admin"))
		assert.True(t, c.HasRole("viewer"))
	})

	t.Run("returns false when role is absent", func(t *testing.T) {
		c := &Claims{Roles: []string{"viewer"}}
		assert.False(t, c.HasRole("admin"))
	})

	t.Run("returns false for empty roles", func(t *testing.T) {
		c := &Claims{}
		assert.False(t, c.HasRole("admin"))
	})
}

func TestClaimsFromContext(t *testing.T) {
	t.Run("returns claims when stored in context", func(t *testing.T) {
		claims := &Claims{Subject: "user-1", Scopes: []string{"read"}}
		ctx := context.WithValue(context.Background(), ContextKey{}, claims)

		got, ok := ClaimsFromContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, claims, got)
	})

	t.Run("returns false when no claims in context", func(t *testing.T) {
		got, ok := ClaimsFromContext(context.Background())

		assert.False(t, ok)
		assert.Nil(t, got)
	})
}
