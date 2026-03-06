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

package core

import (
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

type TMCoreServiceAssembly struct {
	system.DefaultServiceAssembly
	vpaGenerator participantGenerator
}

func (a *TMCoreServiceAssembly) Name() string {
	return "Tenant Manager Core"
}

func (a *TMCoreServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{
		api.ProvisionClientKey,
		store.TransactionContextKey,
		api.ParticipantProfileStoreKey,
		api.DataspaceProfileStoreKey,
		api.CellStoreKey}
}

func (a *TMCoreServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{api.ParticipantProfileServiceKey, api.CellServiceKey, api.DataspaceProfileServiceKey}
}

func (a *TMCoreServiceAssembly) Init(context *system.InitContext) error {
	a.vpaGenerator = participantGenerator{
		CellSelector: defaultCellSelector, // Register the default selector, which may be overridden
	}

	trxContext := context.Registry.Resolve(store.TransactionContextKey).(store.TransactionContext)
	provisionClient := context.Registry.Resolve(api.ProvisionClientKey).(api.ProvisionClient)
	participantStore := context.Registry.Resolve(api.ParticipantProfileStoreKey).(store.EntityStore[*api.ParticipantProfile])
	cellStore := context.Registry.Resolve(api.CellStoreKey).(store.EntityStore[*api.Cell])
	dataspaceStore := context.Registry.Resolve(api.DataspaceProfileStoreKey).(store.EntityStore[*api.DataspaceProfile])
	tenantStore := context.Registry.Resolve(api.TenantStoreKey).(store.EntityStore[*api.Tenant])

	tenantService := tenantService{
		trxContext:       trxContext,
		tenantStore:      tenantStore,
		participantStore: participantStore,
		monitor:          context.LogMonitor,
	}
	context.Registry.Register(api.TenantServiceKey, tenantService)

	participantService := participantService{
		participantGenerator: a.vpaGenerator,
		provisionClient:      provisionClient,
		trxContext:           trxContext,
		participantStore:     participantStore,
		dataspaceStore:       dataspaceStore,
		cellStore:            cellStore,
		monitor:              context.LogMonitor,
	}
	context.Registry.Register(api.ParticipantProfileServiceKey, participantService)

	context.Registry.Register(api.CellServiceKey, cellService{
		trxContext: trxContext,
		cellStore:  cellStore,
	})

	context.Registry.Register(api.DataspaceProfileServiceKey, dataspaceProfileService{
		trxContext:   trxContext,
		profileStore: dataspaceStore,
		cellStore:    cellStore,
	})

	registry := context.Registry.Resolve(api.ProvisionHandlerRegistryKey).(api.ProvisionHandlerRegistry)
	deploymentHandler := vpaCallbackHandler{
		trxContext:       trxContext,
		participantStore: participantStore,
		monitor:          context.LogMonitor,
	}
	registry.Register(model.VPADeployType, deploymentHandler.handleDeploy)
	registry.Register(model.VPADisposeType, deploymentHandler.handleDispose)

	return nil
}

func (a *TMCoreServiceAssembly) Prepare(context *system.InitContext) error {
	selector, found := context.Registry.ResolveOptional(api.CellSelectorKey)
	if found {
		// Override the default selector with a custom implementation
		a.vpaGenerator = selector.(participantGenerator)
	}
	return nil
}
