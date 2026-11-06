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

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/types"
)

// OwnerField is the field that holds the subject of the client that owns a tenant or participant profile.
const OwnerField = "owner"

type tenantAccessKey struct{}

// TenantAccess describes which tenants, and which participant profiles of those tenants, a caller may see and change.
// A caller either has access to all tenants, or only to the tenants it owns.
type TenantAccess struct {
	subject string
	all     bool
}

// AllTenantsAccess grants access to every tenant. The subject is recorded as the owner of tenants the caller creates
// and may be empty.
func AllTenantsAccess(subject string) TenantAccess {
	return TenantAccess{subject: subject, all: true}
}

// OwnedTenantsAccess grants access only to the tenants owned by the given subject. The subject must not be empty, as
// tenants without an owner would otherwise match.
func OwnedTenantsAccess(subject string) TenantAccess {
	return TenantAccess{subject: subject}
}

// Subject returns the subject that is recorded as the owner of tenants the caller creates.
func (a TenantAccess) Subject() string {
	return a.subject
}

// Allows reports whether the caller may access a tenant or participant profile with the given owner.
func (a TenantAccess) Allows(owner string) bool {
	return a.all || (a.subject != "" && owner == a.subject)
}

// Restrict narrows a predicate to the tenants or participant profiles the caller may access.
func (a TenantAccess) Restrict(predicate query.Predicate) query.Predicate {
	if a.all {
		return predicate
	}
	ownerPredicate := query.Eq(OwnerField, a.subject)
	if predicate == nil {
		return ownerPredicate
	}
	return query.And(predicate, ownerPredicate)
}

// IsRestricted reports whether the caller is limited to the tenants it owns.
func (a TenantAccess) IsRestricted() bool {
	return !a.all
}

// ContextWithTenantAccess returns a context that carries the caller's tenant access.
func ContextWithTenantAccess(ctx context.Context, access TenantAccess) context.Context {
	return context.WithValue(ctx, tenantAccessKey{}, access)
}

// TenantAccessFromContext returns the caller's tenant access. It fails when the context carries none, so that a code
// path that forgets to set it is refused instead of seeing every tenant.
func TenantAccessFromContext(ctx context.Context) (TenantAccess, error) {
	access, ok := ctx.Value(tenantAccessKey{}).(TenantAccess)
	if !ok {
		return TenantAccess{}, types.NewFatalError("no tenant access in context")
	}
	return access, nil
}
