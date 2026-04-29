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

	"github.com/eclipse-cfm/cfm/agent/edcv"
	"github.com/eclipse-cfm/cfm/agent/edcv/controlplane"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	. "github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// STSClientIDKey is the context key written by the ih-agent and read here
	STSClientIDKey = "ih.sts.clientId"
)

type EDCVActivityProcessor struct {
	api.BaseActivityProcessor
	VaultClient         serviceapi.VaultClient
	Monitor             system.LogMonitor
	ManagementAPIClient controlplane.ManagementAPIClient
	TokenURL            string
	VaultURL            string
	STSTokenURL         string
	tracer              trace.Tracer
}

type edcData struct {
	ParticipantID       string `json:"cfm.participant.id" validate:"required"`
	VaultAccessClientID string `json:"clientID.vaultAccess" validate:"required"`
	ApiAccessClientID   string `json:"clientID.apiAccess" validate:"required"`
	// CredentialServiceURL the URL of the credential service, i.e., the query and storage endpoints of IdentityHub
	CredentialServiceURL string `json:"cfm.participant.credentialservice"`
	// ProtocolServiceURL the URL of the protocol service, i.e., the DSP protocol endpoint of the control plane
	ProtocolServiceURL string `json:"cfm.participant.protocolservice"`
}

func NewProcessor(config *Config) *EDCVActivityProcessor {
	return &EDCVActivityProcessor{
		VaultClient:         config.VaultClient,
		Monitor:             config.LogMonitor,
		ManagementAPIClient: config.ManagementAPIClient,
		TokenURL:            config.TokenURL,
		VaultURL:            config.VaultURL,
		STSTokenURL:         config.STSTokenURL,
		tracer:              otel.GetTracerProvider().Tracer("cfm.agent.edcv"),
	}
}

type Config struct {
	serviceapi.VaultClient
	system.LogMonitor
	controlplane.ManagementAPIClient
	TokenURL    string
	VaultURL    string
	STSTokenURL string
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

	participantContextId := data.ApiAccessClientID
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextId))
	return p.handleDeployAction(ctx, data, participantContextId)
}

func (p EDCVActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	var data edcData
	err := ctx.ReadValues(&data)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing EDC-V activity for orchestration %s: %w", ctx.OID(), err)}
	}
	return p.handleDisposeAction(ctx.Context(), data.ApiAccessClientID)
}

// handleDeployAction creates the participant context and config in the EDC control plane.
// It expects the STSClientID to already be present in the activity context (written by the ih-agent).
func (p EDCVActivityProcessor) handleDeployAction(ctx api.ActivityContext, data edcData, participantContextId string) api.ActivityResult {

	stsClientIDVal, ok := ctx.Value(STSClientIDKey)
	if !ok || stsClientIDVal == nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("'%s' not found in activity context for orchestration %s — ensure the ih-agent runs before the edcv-agent", STSClientIDKey, ctx.OID())}
	}
	stsClientID, ok := stsClientIDVal.(string)
	if !ok || stsClientID == "" {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("'%s' in activity context is not a valid string for orchestration %s", STSClientIDKey, ctx.OID())}
	}

	did := data.ParticipantID
	if !strings.HasPrefix(did, "did:web:") {
		p.Monitor.Warnf("Participant identifiers are expected to be Web-DIDs, but this one was not: '%s'. Subsequent communication may be severely impacted!", did)
	}

	// resolve vault credentials for the control plane config
	vaultAccessSecret, err := p.VaultClient.ResolveSecret(ctx.Context(), data.VaultAccessClientID)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error retrieving client secret for orchestration %s: %w", ctx.OID(), err)}
	}

	vaultCreds := edcv.VaultCredentials{
		ClientID:     data.VaultAccessClientID,
		ClientSecret: vaultAccessSecret,
		TokenURL:     p.TokenURL,
	}
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
	alias := participantContextId + "-sts-client-secret"
	config := controlplane.NewParticipantContextConfig(participantContextId, stsClientID, alias, data.ParticipantID, vaultConfig, vaultCreds, p.STSTokenURL)
	if err := p.ManagementAPIClient.CreateConfig(ctx.Context(), participantContextId, config); err != nil {
		ctrl.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot create participant config in control plane: %w", err)}
	}
	ctrl.AddEvent("Created ParticipantContextConfig in Control Plane")
	ctrl.End()
	p.Monitor.Infof("EDCV activity for participant '%s' (client ID = %s) completed successfully", data.ParticipantID, data.ApiAccessClientID)

	// delete the vault access secret, since it's no longer needed'
	if err := p.VaultClient.DeleteSecret(ctx.Context(), data.VaultAccessClientID); err != nil {
		p.Monitor.Warnf("failed to delete secret '%s': %v", data.VaultAccessClientID, err)
	}

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
