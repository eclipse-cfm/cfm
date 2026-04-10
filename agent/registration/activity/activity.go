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

	"github.com/eclipse-cfm/cfm/agent/common/issuerservice"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type RegistrationActivityProcessor struct {
	api.BaseActivityProcessor
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

func (p RegistrationActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	tracer := otel.GetTracerProvider().Tracer("cfm.agent.registration")
	_, span := tracer.Start(ctx.Context(), "cfm.agent.registration.create-holder", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	var registrationData RegistrationData
	if err := ctx.ReadValues(&registrationData); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Registration activity for orchestration %s: %w", ctx.OID(), err)}
	}
	if registrationData.HolderName == "" {
		registrationData.HolderName = registrationData.DID
	}

	holderID := registrationData.DID
	if err := p.IssuerService.CreateHolder(ctx.Context(), registrationData.DID, holderID, registrationData.HolderName); err != nil {
		span.RecordError(err)
		// todo: inspect error if it is retryable
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error creating holder in ApiClient: %w", err)}
	}

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p RegistrationActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	var registrationData RegistrationData
	if err := ctx.ReadValues(&registrationData); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Registration activity for orchestration %s: %w", ctx.OID(), err)}
	}
	holderID := registrationData.DID
	if err := p.IssuerService.DeleteHolder(ctx.Context(), holderID); err != nil {
		// todo: inspect error if it is retryable
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("registration rollback: error deleting holder in IssuerService: %w", err)}
	}
	p.Monitor.Debugf("Registration rollback: activity for participant '%s' completed successfully", registrationData.DID)
	return api.ActivityResult{Result: api.ActivityResultComplete}
}
