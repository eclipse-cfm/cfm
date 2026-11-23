/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

// Package vpa connects agents that deploy and dispose virtual participant agents (VPAs) to the engine's activity API.
// The engine calls a single ActivityProcessor.Process method for every activity. Agents that deploy and dispose VPAs
// implement DeployDisposeProcessor instead and register AsActivityProcessor(processor) with the engine.
package vpa

import (
	"fmt"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/pmanager/api"
)

const (
	// DeployDiscriminator is the discriminator of activities that run as part of a VPA deploy orchestration.
	DeployDiscriminator = api.Discriminator(model.VPADeployType)

	// DisposeDiscriminator is the discriminator of activities that run as part of a VPA dispose orchestration.
	DisposeDiscriminator = api.Discriminator(model.VPADisposeType)
)

// DeployDisposeProcessor handles the activities of the VPA deploy and dispose orchestrations.
// ProcessDeploy handles an activity of a VPA deploy orchestration.
// ProcessDispose handles an activity of a VPA dispose orchestration.
type DeployDisposeProcessor interface {
	ProcessDeploy(activityContext api.ActivityContext) api.ActivityResult
	ProcessDispose(activityContext api.ActivityContext) api.ActivityResult
}

// AsActivityProcessor returns an api.ActivityProcessor that calls ProcessDeploy for the deploy discriminator and ProcessDispose for
// the dispose discriminator. An activity with any other discriminator fails with a fatal error.
func AsActivityProcessor(p DeployDisposeProcessor) api.ActivityProcessor {
	return adapter{processor: p}
}

type adapter struct {
	processor DeployDisposeProcessor
}

func (a adapter) Process(ctx api.ActivityContext) api.ActivityResult {
	switch ctx.Discriminator() {
	case DeployDiscriminator:
		return a.processor.ProcessDeploy(ctx)
	case DisposeDiscriminator:
		return a.processor.ProcessDispose(ctx)
	default:
		return api.ActivityResult{
			Result: api.ActivityResultFatalError,
			Error:  fmt.Errorf("the '%s' discriminator is not supported", ctx.Discriminator()),
		}
	}
}

// Properties returns the properties for the given VPA type from the activity context. If the `cfm.vpa.data` value is
// not present, or it does not contain an entry for the VPA type, nil is returned. Otherwise the contents of the
// `properties` field of the matching entry are returned. If the `cfm.vpa.data` value is not a slice, an error is
// returned.
func Properties(ctx api.ActivityContext, vpaType model.VPAType) (map[string]any, error) {
	vpaData, ok := ctx.Value(model.VPAData)
	if !ok || vpaData == nil {
		return nil, nil
	}
	vpaList, ok := vpaData.([]any)
	if !ok {
		return nil, fmt.Errorf("vpa data is not a slice")
	}

	for _, item := range vpaList {
		vpaEntry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entryType, exists := vpaEntry["vpaType"]; exists && entryType == vpaType.String() {
			if properties, ok := vpaEntry["properties"].(map[string]any); ok {
				return properties, nil
			}
			return make(map[string]any), nil
		}
	}

	return nil, nil
}
