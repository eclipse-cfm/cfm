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
	"net/http"
	"strings"

	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"github.com/eclipse-cfm/cfm/agent/edcv"
	"github.com/eclipse-cfm/cfm/agent/edcv/controlplane"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	. "github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/token"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type EDCVActivityProcessor struct {
	api.BaseActivityProcessor
	VaultClient          serviceapi.VaultClient
	HTTPClient           *http.Client
	Monitor              system.LogMonitor
	TokenProvider        token.TokenProvider
	IdentityAPIClient    identityhub.IdentityAPIClient
	TokenURL             string
	VaultURL             string
	STSTokenURL          string
	CredentialServiceURL string
	ProtocolServiceURL   string
	ManagementAPIClient  controlplane.ManagementAPIClient
	tracer               trace.Tracer
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
		VaultClient:          config.VaultClient,
		HTTPClient:           config.Client,
		Monitor:              config.LogMonitor,
		IdentityAPIClient:    config.IdentityAPIClient,
		ManagementAPIClient:  config.ManagementAPIClient,
		TokenURL:             config.TokenURL,
		VaultURL:             config.VaultURL,
		STSTokenURL:          config.STSTokenURL,
		CredentialServiceURL: config.CredentialServiceURL,
		ProtocolServiceURL:   config.ProtocolServiceURL,
		tracer:               otel.GetTracerProvider().Tracer("cfm.agent.edcv"),
	}
}

type Config struct {
	serviceapi.VaultClient
	*http.Client
	system.LogMonitor
	identityhub.IdentityAPIClient
	controlplane.ManagementAPIClient
	TokenURL             string
	VaultURL             string
	STSTokenURL          string
	CredentialServiceURL string
	ProtocolServiceURL   string
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

// handleDeployAction creates a participant context in IdentityHub and the control plane (incl. participant context config)
func (p EDCVActivityProcessor) handleDeployAction(ctx api.ActivityContext, data edcData, participantContextId string) api.ActivityResult {

	// override if config is provided
	if p.CredentialServiceURL != "" {
		data.CredentialServiceURL = fmt.Sprintf(p.CredentialServiceURL, participantContextId)
	}
	if p.ProtocolServiceURL != "" {
		data.ProtocolServiceURL = fmt.Sprintf(p.ProtocolServiceURL, participantContextId)
	}

	if data.CredentialServiceURL == "" {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("CredentialServiceURL is empty")}
	}
	if data.ProtocolServiceURL == "" {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("ProtocolServiceURL is empty")}
	}

	// resolve vault client secret for the new participant
	vaultAccessSecret, err := p.VaultClient.ResolveSecret(ctx.Context(), data.VaultAccessClientID)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error retrieving client secret for orchestration %s: %w", ctx.OID(), err)}
	}
	// create participant-context in IdentityHub
	did := data.ParticipantID

	if !strings.HasPrefix(did, "did:web:") {
		p.Monitor.Warnf("Participant identifiers are expected to be Web-DIDs, but this one was not: '%s'. Subsequent communication may be severely impacted!", did)
	}

	vaultCreds := edcv.VaultCredentials{
		ClientID:     data.VaultAccessClientID,
		ClientSecret: vaultAccessSecret,
		TokenURL:     p.TokenURL,
	}

	_, identyHubSpan := p.tracer.Start(ctx.Context(), "cfm.agent.edcv.deploy.identityhub", trace.WithSpanKind(trace.SpanKindClient))

	manifest := identityhub.NewParticipantManifest(participantContextId, did, data.CredentialServiceURL, data.ProtocolServiceURL, func(m *identityhub.ParticipantManifest) {
		m.VaultCredentials = vaultCreds
		m.VaultConfig.VaultURL = p.VaultURL
		m.VaultConfig.FolderPath = participantContextId + "/identityhub"
		m.CredentialServiceURL = data.CredentialServiceURL
		m.ProtocolServiceURL = data.ProtocolServiceURL
	})
	createResponse, err := p.IdentityAPIClient.CreateParticipantContext(ctx.Context(), manifest)
	if err != nil {
		identyHubSpan.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot create participant in identity hub: %w", err)}
	}
	identyHubSpan.End()

	vaultConfig := manifest.VaultConfig

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
	config := controlplane.NewParticipantContextConfig(participantContextId, createResponse.STSClientID, alias, data.ParticipantID, vaultConfig, vaultCreds, p.STSTokenURL)
	if err := p.ManagementAPIClient.CreateConfig(ctx.Context(), participantContextId, config); err != nil {
		ctrl.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot create participant config in control plane: %w", err)}
	}
	ctrl.AddEvent("Created ParticipantContextConfig in Control Plane")
	ctrl.End()
	p.Monitor.Infof("EDCV activity for participant '%s' (client ID = %s) completed successfully", data.ParticipantID, data.ApiAccessClientID)
	if err := p.VaultClient.DeleteSecret(ctx.Context(), data.VaultAccessClientID); err != nil {
		p.Monitor.Warnf("failed to delete secret '%s': %v", data.VaultAccessClientID, err)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// handleDisposeAction deletes the participant context in IdentityHub and the control plane
func (p EDCVActivityProcessor) handleDisposeAction(ctx context.Context, participantContextID string) api.ActivityResult {
	var errors []error
	// delete from IdentityHub
	err := p.IdentityAPIClient.DeleteParticipantContext(ctx, participantContextID)
	if err != nil {
		errors = append(errors, err)
	}

	// delete config from Control Plane
	err = p.ManagementAPIClient.DeleteConfig(ctx, participantContextID)
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

// extractWebDid extracts a WebDID from a given URL. Currently not used, as the participant profile contains an "identifier" which is the DID.
func (p EDCVActivityProcessor) extractWebDid(url string) (string, error) {

	did := strings.Replace(url, "https", "http", -1)
	did = strings.Replace(did, "http://", "", -1)
	did = strings.Replace(did, ":", "%3A", 8)
	did = strings.ReplaceAll(did, "/", ":")
	did = "did:web:" + did

	return did, nil
}
