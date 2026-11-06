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

package api

import (
	"context"
	"testing"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTenantAccess_Allows(t *testing.T) {
	all := AllTenantsAccess("")
	assert.True(t, all.Allows(""))
	assert.True(t, all.Allows("client-a"))

	owned := OwnedTenantsAccess("client-a")
	assert.True(t, owned.Allows("client-a"))
	assert.False(t, owned.Allows("client-b"))
	assert.False(t, owned.Allows(""), "tenants without an owner are admin-only")

	assert.False(t, OwnedTenantsAccess("").Allows(""), "an empty subject owns nothing")
}

func TestTenantAccess_Restrict(t *testing.T) {
	predicate := query.Eq("id", "tenant-1")

	assert.Same(t, predicate, AllTenantsAccess("operator").Restrict(predicate))

	restricted := OwnedTenantsAccess("client-a").Restrict(predicate)
	assert.Equal(t, query.And(predicate, query.Eq(OwnerField, "client-a")), restricted)

	assert.Equal(t, query.Eq(OwnerField, "client-a"), OwnedTenantsAccess("client-a").Restrict(nil))
}

func TestTenantAccessFromContext(t *testing.T) {
	_, err := TenantAccessFromContext(context.Background())
	require.Error(t, err)
	assert.True(t, types.IsFatal(err))

	ctx := ContextWithTenantAccess(context.Background(), OwnedTenantsAccess("client-a"))
	access, err := TenantAccessFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, "client-a", access.Subject())
}
