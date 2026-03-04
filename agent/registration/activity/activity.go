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

package activity

import (
	"fmt"

	"github.com/metaform/connector-fabric-manager/agent/common/issuerservice"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
)

type RegistrationActivityProcessor struct {
	Monitor       system.LogMonitor
	IssuerService issuerservice.ApiClient
}

type RegistrationData struct {
	DID        string `json:"cfm.participant.id" validate:"required"`
	HolderName string `json:"cfm.participant.holdername"`
}

func NewProcessor(config *Config) *RegistrationActivityProcessor {
	return &RegistrationActivityProcessor{
		Monitor:       config.LogMonitor,
		IssuerService: config.IssuerService,
	}
}

type Config struct {
	system.LogMonitor
	IssuerService issuerservice.ApiClient
}

func (p RegistrationActivityProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	var registrationData RegistrationData
	if err := ctx.ReadValues(&registrationData); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Registration activity for orchestration %s: %w", ctx.OID(), err)}
	}

	if ctx.Discriminator() == api.DeployDiscriminator {
		return p.handleDeployAction(registrationData)
	} else if ctx.Discriminator() == api.DisposeDiscriminator {
		return p.handleDisposeAction(registrationData)
	}
	return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("the '%s' discriminator is not supported", ctx.Discriminator())}

}

func (p RegistrationActivityProcessor) handleDeployAction(registrationData RegistrationData) api.ActivityResult {
	if registrationData.HolderName == "" {
		registrationData.HolderName = registrationData.DID
	}

	if err := p.IssuerService.CreateHolder(registrationData.DID, registrationData.DID, registrationData.HolderName); err != nil {
		// todo: inspect error if it is retryable
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error creating holder in ApiClient: %w", err)}
	}

	p.Monitor.Infof("Registration activity for participant '%s' completed successfully", registrationData.DID)
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p RegistrationActivityProcessor) handleDisposeAction(data RegistrationData) api.ActivityResult {

	if err := p.IssuerService.DeleteHolder(data.DID); err != nil {
		// todo: inspect error if it is retryable
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error deleting holder in ApiClient: %w", err)}
	}
	p.Monitor.Infof("Registration compensation activity for participant '%s' completed successfully", data.DID)
	return api.ActivityResult{Result: api.ActivityResultComplete}
}
