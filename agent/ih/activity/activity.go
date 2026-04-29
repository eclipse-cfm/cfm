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

	commonvault "github.com/eclipse-cfm/cfm/agent/common/vault"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"

	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// STSClientIDKey is the key under which the STS client ID returned by IdentityHub is stored in the
// activity processing data, so that downstream agents (e.g. the edcv-agent) can read it.
const STSClientIDKey = "ih.sts.clientId"

type IdentityHubActivityProcessor struct {
	api.BaseActivityProcessor
	VaultClient       serviceapi.VaultClient
	HTTPClient        *http.Client
	Monitor           system.LogMonitor
	IdentityAPIClient identityhub.IdentityAPIClient
	TokenURL          string
	VaultURL          string
	// CredentialServiceURL optional template for the credential service URL; use %s as placeholder for participantContextID
	CredentialServiceURL string
	// ProtocolServiceURL optional template for the protocol service URL; use %s as placeholder for participantContextID
	ProtocolServiceURL string
	tracer             trace.Tracer
}

type ihData struct {
	ParticipantID        string `json:"cfm.participant.id" validate:"required"`
	VaultAccessClientID  string `json:"clientID.vaultAccess" validate:"required"`
	ApiAccessClientID    string `json:"clientID.apiAccess" validate:"required"`
	CredentialServiceURL string `json:"cfm.participant.credentialservice"`
	ProtocolServiceURL   string `json:"cfm.participant.protocolservice"`
}

func NewProcessor(config *Config) *IdentityHubActivityProcessor {
	return &IdentityHubActivityProcessor{
		VaultClient:          config.VaultClient,
		HTTPClient:           config.Client,
		Monitor:              config.LogMonitor,
		IdentityAPIClient:    config.IdentityAPIClient,
		TokenURL:             config.TokenURL,
		VaultURL:             config.VaultURL,
		CredentialServiceURL: config.CredentialServiceURL,
		ProtocolServiceURL:   config.ProtocolServiceURL,
		tracer:               otel.GetTracerProvider().Tracer("cfm.agent.identityhub"),
	}
}

type Config struct {
	serviceapi.VaultClient
	*http.Client
	system.LogMonitor
	identityhub.IdentityAPIClient
	TokenURL             string
	VaultURL             string
	CredentialServiceURL string
	ProtocolServiceURL   string
}

func (p IdentityHubActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	_, span := p.tracer.Start(ctx.Context(), "cfm.agent.identityhub.deploy")
	defer span.End()

	var data ihData
	if err := ctx.ReadValues(&data); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing IH activity for orchestration %s: %w", ctx.OID(), err)}
	}

	participantContextId := data.ApiAccessClientID
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextId))
	return p.handleDeployAction(ctx, data, participantContextId)
}

func (p IdentityHubActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	var data ihData
	if err := ctx.ReadValues(&data); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing IH activity for orchestration %s: %w", ctx.OID(), err)}
	}
	return p.handleDisposeAction(ctx.Context(), data.ApiAccessClientID)
}

// handleDeployAction creates the participant context in IdentityHub and stores the returned STSClientID
// in the activity context so that downstream agents can use it.
func (p IdentityHubActivityProcessor) handleDeployAction(ctx api.ActivityContext, data ihData, participantContextId string) api.ActivityResult {

	// apply URL templates if configured
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

	did := data.ParticipantID
	if !strings.HasPrefix(did, "did:web:") {
		p.Monitor.Warnf("Participant identifiers are expected to be Web-DIDs, but this one was not: '%s'. Subsequent communication may be severely impacted!", did)
	}

	// resolve vault access secret for the new participant
	vaultAccessSecret, err := p.VaultClient.ResolveSecret(ctx.Context(), data.VaultAccessClientID)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error retrieving client secret for orchestration %s: %w", ctx.OID(), err)}
	}

	vaultCreds := commonvault.Credentials{
		ClientID:     data.VaultAccessClientID,
		ClientSecret: vaultAccessSecret,
		TokenURL:     p.TokenURL,
	}

	_, ihSpan := p.tracer.Start(ctx.Context(), "cfm.agent.identityhub.deploy", trace.WithSpanKind(trace.SpanKindClient))

	manifest := identityhub.NewParticipantManifest(participantContextId, did, data.CredentialServiceURL, data.ProtocolServiceURL, func(m *identityhub.ParticipantManifest) {
		m.VaultCredentials = vaultCreds
		m.VaultConfig.VaultURL = p.VaultURL
		m.VaultConfig.FolderPath = participantContextId + "/identityhub"
	})
	createResponse, err := p.IdentityAPIClient.CreateParticipantContext(ctx.Context(), manifest)
	if err != nil {
		ihSpan.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot create participant in IdentityHub: %w", err)}
	}
	ihSpan.End()

	// make the STS client ID available to downstream agents
	ctx.SetValue(STSClientIDKey, createResponse.STSClientID)

	p.Monitor.Infof("IH activity for participant '%s' (client ID = %s) completed successfully", data.ParticipantID, participantContextId)
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// handleDisposeAction deletes the participant context from IdentityHub
func (p IdentityHubActivityProcessor) handleDisposeAction(ctx context.Context, participantContextID string) api.ActivityResult {
	err := p.IdentityAPIClient.DeleteParticipantContext(ctx, participantContextID)
	if err != nil {
		p.Monitor.Warnf("error deleting participant context '%s' from IdentityHub: %v", participantContextID, err)
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}
