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

package api

import (
	"context"
	"iter"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
)

const (
	TenantServiceKey             system.ServiceType = "tmapi:TenantService"
	ParticipantProfileServiceKey system.ServiceType = "tmapi:ParticipantProfileService"
	DataspaceProfileServiceKey   system.ServiceType = "tmapi:DataspaceProfileService"
	CellServiceKey               system.ServiceType = "tmapi:CellService"
)

// TenantService performs tenant operations.
type TenantService interface {
	GetTenant(ctx context.Context, tenantID string) (*Tenant, error)
	CreateTenant(ctx context.Context, tenant *Tenant) (*Tenant, error)
	DeleteTenant(ctx context.Context, tenantID string) error
	PatchTenant(ctx context.Context, id string, properties map[string]any, remove []string) error
	GetTenants(ctx context.Context, options store.PaginationOptions) iter.Seq2[*Tenant, error]
	GetTenantsCount(ctx context.Context) (int64, error)
	QueryTenants(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*Tenant, error]
	QueryTenantsCount(ctx context.Context, predicate query.Predicate) (int64, error)
}

// ParticipantProfileService performs participant profile operations, including deploying associated VPAs.
type ParticipantProfileService interface {
	GetProfile(ctx context.Context, tenantID string, participantID string) (*ParticipantProfile, error)
	QueryProfiles(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*ParticipantProfile, error]
	QueryProfilesCount(ctx context.Context, predicate query.Predicate) (int64, error)
	DeployProfile(ctx context.Context, tenantID string, deployment *NewParticipantProfileDeployment) (*ParticipantProfile, error)
	DisposeProfile(ctx context.Context, tenantID string, participantID string) error
}

// DataspaceProfileService performs dataspace profile operations.
type DataspaceProfileService interface {
	GetProfile(ctx context.Context, profileID string) (*DataspaceProfile, error)
	CreateProfile(ctx context.Context, profile *DataspaceProfile) (*DataspaceProfile, error)
	DeleteProfile(ctx context.Context, profileID string) error
	DeployProfile(ctx context.Context, profileID string, cellID string) error
	ListProfiles(ctx context.Context) ([]DataspaceProfile, error)
}

// CellService performs cell operations.
type CellService interface {
	RecordExternalDeployment(ctx context.Context, cell *Cell) (*Cell, error)
	DeleteCell(ctx context.Context, cellID string) error
	ListCells(ctx context.Context) ([]Cell, error)
}
