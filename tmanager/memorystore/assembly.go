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

package memorystore

import (
	memorystore2 "github.com/eclipse-cfm/cfm/common/memorystore"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

type InMemoryServiceAssembly struct {
	system.DefaultServiceAssembly
}

func (a *InMemoryServiceAssembly) Name() string {
	return "Tenant Manager In-Memory Store"
}

func (a *InMemoryServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{api.CellStoreKey, api.DataspaceProfileStoreKey, api.ParticipantProfileStoreKey}
}

func (a *InMemoryServiceAssembly) Init(ictx *system.InitContext) error {
	cellStore := memorystore2.NewInMemoryEntityStore[*api.Cell]()
	dataspaceStore := memorystore2.NewInMemoryEntityStore[*api.DataspaceProfile]()
	participantStore := memorystore2.NewInMemoryEntityStore[*api.ParticipantProfile]()
	tenantStore := memorystore2.NewInMemoryEntityStore[*api.Tenant]()

	ictx.Registry.Register(api.TenantStoreKey, tenantStore)
	ictx.Registry.Register(api.ParticipantProfileStoreKey, participantStore)
	ictx.Registry.Register(api.DataspaceProfileStoreKey, dataspaceStore)
	ictx.Registry.Register(api.CellStoreKey, cellStore)
	return nil
}
