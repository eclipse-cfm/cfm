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

package activity

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/token"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	participantContextIDKey = "participantContextId"
	jsonContentType         = "application/json"
	contentTypeHeader       = "Content-Type"
	authHeader              = "Authorization"
	clientIdentifier        = "system:serviceaccount:edc-v:cfm-agents"
)

// vaultClientIdentifiers are the workload ServiceAccounts that exchange their projected token for a
// participant-scoped token used to authenticate against Vault. The resulting token's `sub` is the
// participant context id, which scopes the workload to that participant's vault partition.
var vaultClientIdentifiers = []string{
	"system:serviceaccount:edc-v:controlplane",
	"system:serviceaccount:edc-v:identityhub",
}

type TokenExchangeActivityProcessor struct {
	api.BaseActivityProcessor
	Monitor            system.LogMonitor
	TokenProvider      token.TokenProvider
	tracer             trace.Tracer
	HttpClient         *http.Client
	TokenFilePath      string
	Audience           string
	ManagementBasePath string
}

type tokenExchangeData struct {
	ParticipantID string `json:"cfm.participant.id" validate:"required"`
}

type resourceMapping struct {
	ClientIdentifier   string   `json:"clientIdentifier" validate:"required"`
	ParticipantContext string   `json:"participantContext" validate:"required"`
	Scopes             []string `json:"scopes" validate:"required"`
	Audiences          []string `json:"audiences"`
}

type scopeMapping struct {
	Scope  string            `json:"scope" validate:"required"`
	Claims map[string]string `json:"claims"`
}

type Config struct {
	system.LogMonitor
	token.TokenProvider
	HttpClient         *http.Client
	ManagementBasePath string
	TokenFilePath      string
	Audience           string
}

func NewProcessor(config *Config) *TokenExchangeActivityProcessor {
	return &TokenExchangeActivityProcessor{
		Monitor:            config.LogMonitor,
		TokenProvider:      config.TokenProvider,
		HttpClient:         config.HttpClient,
		tracer:             otel.GetTracerProvider().Tracer("cfm.agent.test"),
		ManagementBasePath: config.ManagementBasePath,
		TokenFilePath:      config.TokenFilePath,
		Audience:           config.Audience,
	}
}

func (p TokenExchangeActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {

	var data tokenExchangeData
	if err := ctx.ReadValues(&data); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing token exchange deploy for orchestration %s: %w", ctx.OID(), err)}
	}

	participantContextID := generateClientID()

	_, span := p.tracer.Start(ctx.Context(), "cfm.agent.jwtlet.token-setup", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	// step 1: create resource mapping
	rm := resourceMapping{
		ClientIdentifier:   clientIdentifier,
		ParticipantContext: participantContextID,
		Scopes:             []string{"read", "write", "admin"},
		Audiences:          []string{p.Audience},
	}
	p.Monitor.Debugf("Creating resource mapping for participant context: %s", participantContextID)
	err := p.post(ctx.Context(), "/api/v1/mappings", rm)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error creating resource mapping: %w", err)}
	}

	// step 1b: allow the control plane and identity hub workloads to exchange their token for a
	// participant-scoped token used to authenticate against Vault (resource = participant context id).
	for _, clientID := range vaultClientIdentifiers {
		vm := resourceMapping{
			ClientIdentifier:   clientID,
			ParticipantContext: participantContextID,
			Scopes:             []string{"read"},
			Audiences:          []string{p.Audience},
		}
		p.Monitor.Debugf("Creating vault resource mapping for %s -> %s", clientID, participantContextID)
		if err := p.post(ctx.Context(), "/api/v1/mappings", vm); err != nil {
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error creating vault resource mapping for %s: %w", clientID, err)}
		}
	}

	// step 2: test token exchange
	p.Monitor.Debugf("testing token exchange for participant context: %s", participantContextID)
	scopedToken, err := p.TokenProvider.GetToken(ctx.Context(), "read write", participantContextID)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("testing token exchange failed: error getting scoped token: %w", err)}
	}
	claims, err := decodeJWTClaims(scopedToken)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("testing token exchange failed: error decoding JWT claims: %w", err)}
	}
	p.Monitor.Debugf("token exchange successful. claims: %s", claims)

	// set both - for further processing in other agents and for the output
	ctx.SetValue(participantContextIDKey, participantContextID)
	ctx.SetOutputValue(participantContextIDKey, participantContextID)

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p TokenExchangeActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	participantContextID, ok := ctx.Value(participantContextIDKey)
	if !ok {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing token exchange dispose for orchestration %s: participant context ID not found in context", ctx.OID())}
	}
	err := p.delete(ctx.Context(), fmt.Sprintf("/api/v1/mappings/%s/%s", clientIdentifier, participantContextID))
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error deleting resource mapping: %w", err)}
	}

	for _, clientID := range vaultClientIdentifiers {
		if err := p.delete(ctx.Context(), fmt.Sprintf("/api/v1/mappings/%s/%s", clientID, participantContextID)); err != nil {
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error deleting vault resource mapping for %s: %w", clientID, err)}
		}
	}

	// we do NOT delete the scope mappings, because they only exist once for all participants

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p TokenExchangeActivityProcessor) post(ctx context.Context, url string, body any) error {
	// read workload token
	tokenBytes, err := os.ReadFile(p.TokenFilePath)
	if err != nil {
		return fmt.Errorf("error reading token file from %s: %w", p.TokenFilePath, err)
	}

	bodyJson, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error marshalling body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.ManagementBasePath+url, bytes.NewReader(bodyJson))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set(contentTypeHeader, jsonContentType)
	req.Header.Set(authHeader, "Bearer "+string(tokenBytes))
	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request '%s %s' failed with status %d", req.Method, req.URL.String(), resp.StatusCode)
	}

	return nil
}

func (p TokenExchangeActivityProcessor) delete(ctx context.Context, url string) error {
	// read workload token
	tokenBytes, err := os.ReadFile(p.TokenFilePath)
	if err != nil {
		return fmt.Errorf("error reading token file from %s: %w", p.TokenFilePath, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.ManagementBasePath+url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set(authHeader, "Bearer "+string(tokenBytes))
	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request '%s %s' failed with status %d", req.Method, req.URL.String(), resp.StatusCode)
	}

	return nil
}

func generateClientID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// decodeJWTClaims base64-decodes the payload section of a JWT and returns it as a raw JSON string.
func decodeJWTClaims(jwtToken string) (string, error) {
	parts := strings.SplitN(jwtToken, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("error decoding JWT payload: %w", err)
	}
	return string(payload), nil
}
