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
	"context"
	"fmt"
	"strings"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/orchestration/edcv"
	. "github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type EDCVActivityProcessor struct {
	api.BaseActivityProcessor
	Monitor             system.LogMonitor
	ManagementAPIClient controlplane.ManagementAPIClient
	VaultURL            string
	tracer              trace.Tracer
}

type edcData struct {
	ParticipantID        string `json:"cfm.participant.id" validate:"required"`
	ParticipantContextId string `json:"participantContextId" validate:"required"`
	// CredentialServiceURL the URL of the credential service, i.e., the query and storage endpoints of IdentityHub
	CredentialServiceURL string `json:"cfm.participant.credentialservice"`
	// ProtocolServiceURL the URL of the protocol service, i.e., the DSP protocol endpoint of the control plane
	ProtocolServiceURL string `json:"cfm.participant.protocolservice"`
}

func NewProcessor(config *Config) *EDCVActivityProcessor {
	return &EDCVActivityProcessor{
		Monitor:             config.LogMonitor,
		ManagementAPIClient: config.ManagementAPIClient,
		VaultURL:            config.VaultURL,
		tracer:              otel.GetTracerProvider().Tracer("cfm.agent.edcv"),
	}
}

type Config struct {
	system.LogMonitor
	controlplane.ManagementAPIClient
	VaultURL string
}

func (p EDCVActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {

	_, span := p.tracer.Start(ctx.Context(), "cfm.agent.edcv.deploy")
	defer span.End()

	var data edcData
	err := ctx.ReadValues(&data)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing EDC-V activity for orchestration %s: %w", ctx.OID(), err)}
	}

	participantContextId := data.ParticipantContextId
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextId))
	return p.handleDeployAction(ctx, data, participantContextId)
}

func (p EDCVActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	var data edcData
	err := ctx.ReadValues(&data)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing EDC-V activity for orchestration %s: %w", ctx.OID(), err)}
	}
	return p.handleDisposeAction(ctx.Context(), data.ParticipantContextId)
}

// handleDeployAction creates the participant context and config in the EDC control plane.
func (p EDCVActivityProcessor) handleDeployAction(ctx api.ActivityContext, data edcData, participantContextId string) api.ActivityResult {

	did := data.ParticipantID
	if !strings.HasPrefix(did, "did:web:") {
		p.Monitor.Warnf("Participant identifiers are expected to be Web-DIDs, but this one was not: '%s'. Subsequent communication may be severely impacted!", did)
	}

	// the control plane authenticates to Vault via token exchange (jwtlet), so only the vault
	// config (no credentials) is needed; the participant context id is the token-exchange resource.
	vaultConfig := edcv.VaultConfig{
		VaultURL:   p.VaultURL,
		SecretPath: "v1/participants",
		FolderPath: participantContextId + "/identityhub",
	}

	_, ctrl := p.tracer.Start(ctx.Context(), "cfm.agent.edcv.deploy.controlplane", trace.WithSpanKind(trace.SpanKindClient))

	// create participant context in Control Plane
	if err := p.ManagementAPIClient.CreateParticipantContext(ctx.Context(), controlplane.ParticipantContext{
		ParticipantContextID: participantContextId,
		Identifier:           did,
		Properties:           make(map[string]any),
		State:                controlplane.ParticipantContextStateActivated,
	}); err != nil {
		ctrl.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot create participant context in control plane: %w", err)}
	}
	ctrl.AddEvent("Created ParticipantContext in Control Plane")

	// create participant config in Control Plane
	config := controlplane.NewParticipantContextConfig(participantContextId, data.ParticipantID, vaultConfig)
	if err := p.ManagementAPIClient.PatchConfig(ctx.Context(), participantContextId, config); err != nil {
		ctrl.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot create participant config in control plane: %w", err)}
	}
	ctrl.AddEvent("Created ParticipantContextConfig in Control Plane")
	ctrl.End()
	p.Monitor.Infof("EDCV activity for participant '%s' (client ID = %s) completed successfully", data.ParticipantID, data.ParticipantContextId)

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// handleDisposeAction deletes the participant context and config from the EDC control plane
func (p EDCVActivityProcessor) handleDisposeAction(ctx context.Context, participantContextID string) api.ActivityResult {
	var errors []error

	// delete config from Control Plane
	err := p.ManagementAPIClient.DeleteConfig(ctx, participantContextID)
	if err != nil {
		errors = append(errors, err)
	}

	// delete participant context from Control Plane
	err = p.ManagementAPIClient.DeleteParticipantContext(ctx, participantContextID)
	if err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		errorStrings := Collect(Map(From(errors), func(err error) string { return err.Error() }))
		errStr := strings.Join(errorStrings, ", ")
		p.Monitor.Warnf("one or more errors occurred while rolling back participant context '%s': [%s]", participantContextID, errStr)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}
