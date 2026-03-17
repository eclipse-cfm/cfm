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

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
)

const (
	ProvisionHandlerRegistryKey system.ServiceType = "tmapi:ProvisionHandlerRegistry"
	ProvisionClientKey          system.ServiceType = "tmapi:ProvisionClient"
)

type VPAPropMap = map[model.VPAType]map[string]any

// ProvisionClient sends a manifest to the provision manager for asynchronous execution. Implementations may use
// different wire protocols.
type ProvisionClient interface {
	// Send delivers the specified manifest.
	// If a recoverable error is encountered one of model.RecoverableError, model.ClientError, or model.FatalError will be returned.
	Send(ctx context.Context, manifest model.OrchestrationManifest) error
}

// ProvisionCallbackHandler is called when an orchestration is complete.
// If a recoverable error is encountered one of model.RecoverableError, model.ClientError, or model.FatalError will be returned.
type ProvisionCallbackHandler func(context.Context, model.OrchestrationResponse) error

// ProvisionHandlerRegistry registers orchestration handlers by type.
type ProvisionHandlerRegistry interface {
	Register(orchestrationType model.OrchestrationType, handler ProvisionCallbackHandler)
}

func ToVPAMap(vpaProperties map[string]map[string]any) *VPAPropMap {
	vpaPropsMap := make(VPAPropMap)
	for vpaTypeStr, props := range vpaProperties {
		vpaType := model.VPAType(vpaTypeStr)
		vpaPropsMap[vpaType] = props
	}
	return &vpaPropsMap
}
