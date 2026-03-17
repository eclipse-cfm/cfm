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
	"github.com/eclipse-cfm/cfm/common/memorystore"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
)

type MemoryStoreServiceAssembly struct {
	system.DefaultServiceAssembly
}

func (m MemoryStoreServiceAssembly) Name() string {
	return "Provision Manager Memory Store"
}

func (m MemoryStoreServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{api.DefinitionStoreKey, api.OrchestrationIndexKey}
}

func (m MemoryStoreServiceAssembly) Init(context *system.InitContext) error {
	context.Registry.Register(api.DefinitionStoreKey, NewDefinitionStore())
	context.Registry.Register(
		api.OrchestrationIndexKey,
		memorystore.NewInMemoryEntityStore[*api.OrchestrationEntry]())
	return nil
}
