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
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"github.com/eclipse-cfm/cfm/agent/common/issuerservice"
	"github.com/eclipse-cfm/cfm/agent/common/siglet"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/token"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	participantContextIDKey = "participantContextId"
	jsonContentType         = "application/json"
	contentTypeHeader       = "Content-Type"
	authHeader              = "Authorization"
	agentsServiceAccount    = "cfm-agents"

	// scopeVaultRead is the scope every CFM workload needs on its participant-scoped token to
	// authenticate against the participant's Vault partition.
	scopeVaultRead = "read"

	// scopeSignaling is the scope the control plane needs to authorize DPS signaling exchanges with
	// the participant's data plane. It must match the scope the siglet agent puts into the data-plane
	// authorization profile (`tokenexchange.scope`, see agent/orchestration/siglet/launcher/launcher.go).
	scopeSignaling = "signaling"
)

// ClientMapping names a workload ServiceAccount that exchanges its projected token for a
// participant-scoped token, together with the scopes its resource mapping grants.
type ClientMapping struct {
	// Name is the bare ServiceAccount name; the client identifier jwtlet sees is built from it and
	// Config.ServiceAccountNamespace.
	Name   string   `json:"name" mapstructure:"name"`
	Scopes []string `json:"scopes" mapstructure:"scopes"`
}

// defaultClientMappings are the client mappings used when none are configured. The workloads they
// name exchange their projected token for a participant-scoped token whose `sub` is the participant
// context id, which scopes the workload to that participant's vault partition. Every workload gets
// scopeVaultRead; the control plane additionally gets scopeSignaling for DPS signaling with the
// data plane.
var defaultClientMappings = []ClientMapping{
	{Name: "controlplane", Scopes: []string{scopeVaultRead, scopeSignaling}},
	{Name: "identityhub", Scopes: []string{scopeVaultRead}},
	{Name: "siglet-sa", Scopes: []string{scopeVaultRead}},
}

// agentScopes is the exact set of narrow scopes the CFM agents request against a participant
// context, referenced from the API clients so the mapping cannot drift from what they request.
// Every scope listed here must have a (1:1) scope mapping seeded in jwtlet.
var agentScopes = []string{
	controlplane.ScopeApiAdmin,
	identityhub.ScopeApiAdmin,
	identityhub.ScopeCredentialsRead,
	identityhub.ScopeCredentialsWrite,
	issuerservice.ScopeApiAdmin,
	siglet.ScopeApiRead,
	siglet.ScopeApiWrite,
	"read",
	"write",
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

	clientIdentifier string
	resolvedMappings []resolvedMapping
}

// resolvedMapping is a ClientMapping with its ServiceAccount name resolved to the client identifier
// jwtlet sees for the workload.
type resolvedMapping struct {
	clientIdentifier string
	scopes           []string
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
	// ServiceAccountNamespace is the Kubernetes namespace the CFM workload ServiceAccounts live in,
	// used to build the client identifiers of the resource mappings.
	ServiceAccountNamespace string
	// ClientMappings are the workload ServiceAccounts that exchange their projected token for a
	// participant-scoped token, and the scopes each one gets (config key `clientmappings`). When
	// empty, defaultClientMappings is used, so a deployment that does not configure the key keeps
	// the built-in mappings.
	ClientMappings []ClientMapping
}

func NewProcessor(config *Config) *TokenExchangeActivityProcessor {
	mappings := config.ClientMappings
	if len(mappings) == 0 {
		mappings = defaultClientMappings
	}
	resolvedMappings := make([]resolvedMapping, 0, len(mappings))
	for _, mapping := range mappings {
		resolvedMappings = append(resolvedMappings, resolvedMapping{
			clientIdentifier: serviceAccountIdentifier(config.ServiceAccountNamespace, mapping.Name),
			scopes:           mapping.Scopes,
		})
	}
	return &TokenExchangeActivityProcessor{
		Monitor:            config.LogMonitor,
		TokenProvider:      config.TokenProvider,
		HttpClient:         config.HttpClient,
		tracer:             otel.GetTracerProvider().Tracer("cfm.agent.jwtlet"),
		ManagementBasePath: config.ManagementBasePath,
		TokenFilePath:      config.TokenFilePath,
		Audience:           config.Audience,
		clientIdentifier:   serviceAccountIdentifier(config.ServiceAccountNamespace, agentsServiceAccount),
		resolvedMappings:   resolvedMappings,
	}
}

// serviceAccountIdentifier builds the subject Kubernetes puts into a ServiceAccount's projected
// token, which is what jwtlet sees as the client identifier of a workload.
func serviceAccountIdentifier(namespace, name string) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", namespace, name)
}

func (p TokenExchangeActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {

	spanCtx, span := p.tracer.Start(ctx.Context(), "cfm.agent.jwtlet.deploy")
	defer span.End()

	var data tokenExchangeData
	if err := ctx.ReadValues(&data); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing token exchange deploy for orchestration %s: %w", ctx.OID(), err)}
	}

	// The ID is derived from the orchestration, so a retried or concurrently redelivered attempt works on the same
	// participant context instead of creating another one. It is stored right away so that it is persisted even if the
	// activity fails, which lets the dispose step clean up whatever was created.
	participantContextID := deriveParticipantContextID(ctx.OID(), ctx.ID())
	ctx.SetValue(participantContextIDKey, participantContextID)
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextID))

	// step 1: create the resource mappings via the jwtlet management API
	mapCtx, mapSpan := p.tracer.Start(spanCtx, "cfm.agent.jwtlet.deploy.mappings", trace.WithSpanKind(trace.SpanKindClient))

	// the CFM agents must be able to exchange their token for a participant-scoped EDC API token
	rm := resourceMapping{
		ClientIdentifier:   p.clientIdentifier,
		ParticipantContext: participantContextID,
		Scopes:             agentScopes,
		Audiences:          []string{p.Audience},
	}
	p.Monitor.Debugf("Creating resource mapping for participant context: %s", participantContextID)
	if err := p.createOrReplaceMapping(mapCtx, rm); err != nil {
		mapSpan.RecordError(err)
		mapSpan.End()
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error creating resource mapping: %w", err)}
	}
	mapSpan.AddEvent("Created CFM agents resource mapping")

	// the participant's workloads must be able to exchange their token for a participant-scoped token
	// used to authenticate against Vault (resource = participant context id). Which workloads these
	// are, and the scopes they get, comes from the configured client mappings (defaultClientMappings
	// when unconfigured, where the control plane additionally gets the signaling scope to authorize
	// DPS exchanges with the data plane).
	for _, mapping := range p.resolvedMappings {
		wm := resourceMapping{
			ClientIdentifier:   mapping.clientIdentifier,
			ParticipantContext: participantContextID,
			Scopes:             mapping.scopes,
			Audiences:          []string{p.Audience},
		}
		p.Monitor.Debugf("Creating workload resource mapping for %s -> %s", mapping.clientIdentifier, participantContextID)
		if err := p.createOrReplaceMapping(mapCtx, wm); err != nil {
			mapSpan.RecordError(err)
			mapSpan.End()
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error creating resource mapping for %s: %w", mapping.clientIdentifier, err)}
		}
	}
	mapSpan.AddEvent("Created workload resource mappings")
	mapSpan.End()

	// step 2: verify the token exchange works for the new participant context. Requesting the
	// full agent scope set also proves that every scope has a scope mapping seeded in jwtlet,
	// so a missing mapping fails here instead of in a downstream agent.
	exCtx, exSpan := p.tracer.Start(spanCtx, "cfm.agent.jwtlet.deploy.verify-token-exchange", trace.WithSpanKind(trace.SpanKindClient))
	p.Monitor.Debugf("testing token exchange for participant context: %s", participantContextID)
	scopedToken, err := p.TokenProvider.GetToken(exCtx, strings.Join(agentScopes, " "), participantContextID)
	if err != nil {
		exSpan.RecordError(err)
		exSpan.End()
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("testing token exchange failed: error getting scoped token: %w", err)}
	}
	claims, err := decodeJWTClaims(scopedToken)
	if err != nil {
		exSpan.RecordError(err)
		exSpan.End()
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("testing token exchange failed: error decoding JWT claims: %w", err)}
	}
	exSpan.AddEvent("Verified token exchange")
	exSpan.End()
	// Only identifying claims are logged: the others may carry more than an operator needs to see
	p.Monitor.Debugf("token exchange successful for participant context %s: sub=%v, aud=%v", participantContextID, claims["sub"], claims["aud"])

	// the processing data already holds the ID for further processing in other agents; also return it as output
	ctx.SetOutputValue(participantContextIDKey, participantContextID)

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p TokenExchangeActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	spanCtx, span := p.tracer.Start(ctx.Context(), "cfm.agent.jwtlet.dispose", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	value, ok := ctx.Value(participantContextIDKey)
	if !ok {
		err := fmt.Errorf("error processing token exchange dispose for orchestration %s: participant context ID not found in context", ctx.OID())
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	participantContextID, ok := value.(string)
	if !ok {
		err := fmt.Errorf("error processing token exchange dispose for orchestration %s: participant context ID is not a string", ctx.OID())
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	if err := model.ValidateParticipantContextID(participantContextID); err != nil {
		err = fmt.Errorf("error processing token exchange dispose for orchestration %s: %w", ctx.OID(), err)
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextID))

	if err := p.deleteMapping(spanCtx, p.clientIdentifier, participantContextID); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error deleting resource mapping: %w", err)}
	}

	for _, mapping := range p.resolvedMappings {
		if err := p.deleteMapping(spanCtx, mapping.clientIdentifier, participantContextID); err != nil {
			span.RecordError(err)
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error deleting resource mapping for %s: %w", mapping.clientIdentifier, err)}
		}
	}
	span.AddEvent("Deleted resource mappings")

	// we do NOT delete the scope mappings, because they only exist once for all participants

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// createOrReplaceMapping creates a resource mapping, or replaces it if it already exists. A mapping already exists when
// an earlier attempt of the activity created it, so replacing it makes the deploy step safe to repeat.
func (p TokenExchangeActivityProcessor) createOrReplaceMapping(ctx context.Context, mapping resourceMapping) error {
	method := http.MethodPost
	status, err := p.send(ctx, method, "/api/v1/mappings", mapping)
	if err != nil {
		return err
	}
	if status == http.StatusConflict {
		p.Monitor.Debugf("Resource mapping %s -> %s already exists, replacing it", mapping.ClientIdentifier, mapping.ParticipantContext)
		method = http.MethodPut
		status, err = p.send(ctx, method, mappingPath(mapping.ClientIdentifier, mapping.ParticipantContext), mapping)
		if err != nil {
			return err
		}
	}
	return checkStatus(method, mapping.ClientIdentifier, status)
}

// deleteMapping deletes a resource mapping. A mapping that does not exist counts as deleted, so the dispose step is safe
// to repeat and can clean up after a deploy that failed part-way.
func (p TokenExchangeActivityProcessor) deleteMapping(ctx context.Context, clientIdentifier string, participantContextID string) error {
	status, err := p.send(ctx, http.MethodDelete, mappingPath(clientIdentifier, participantContextID), nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		p.Monitor.Debugf("Resource mapping %s -> %s does not exist, nothing to delete", clientIdentifier, participantContextID)
		return nil
	}
	return checkStatus(http.MethodDelete, clientIdentifier, status)
}

// send sends a request to the jwtlet management API, with body as JSON unless it is nil, and returns the response
// status.
func (p TokenExchangeActivityProcessor) send(ctx context.Context, method string, path string, body any) (int, error) {
	// read workload token
	tokenBytes, err := os.ReadFile(p.TokenFilePath)
	if err != nil {
		return 0, fmt.Errorf("error reading token file from %s: %w", p.TokenFilePath, err)
	}

	var reqBody io.Reader
	if body != nil {
		bodyJson, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("error marshalling body: %w", err)
		}
		reqBody = bytes.NewReader(bodyJson)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.ManagementBasePath+path, reqBody)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %w", err)
	}
	if body != nil {
		req.Header.Set(contentTypeHeader, jsonContentType)
	}
	req.Header.Set(authHeader, "Bearer "+string(tokenBytes))
	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error sending request '%s %s': %w", method, req.URL.String(), err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// checkStatus returns an error unless status is a success status.
func checkStatus(method string, clientIdentifier string, status int) error {
	if status < 200 || status >= 300 {
		return fmt.Errorf("request '%s' for the resource mapping of %s failed with status %d", method, clientIdentifier, status)
	}
	return nil
}

// mappingPath returns the jwtlet management API path of the resource mapping for a client and participant context.
func mappingPath(clientIdentifier string, participantContextID string) string {
	return "/api/v1/mappings/" + url.PathEscape(clientIdentifier) + "/" + url.PathEscape(participantContextID)
}

// participantContextIDNamespace is the namespace of the name-based UUIDs participant context IDs are derived from.
var participantContextIDNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://github.com/eclipse-cfm/cfm/jwtlet/participant-context"))

// deriveParticipantContextID returns the participant context ID for the given activity of an orchestration: a
// name-based UUID without dashes, so every attempt of the activity gets the same ID while different orchestrations get
// different ones.
func deriveParticipantContextID(orchestrationID string, activityID string) string {
	id := uuid.NewSHA1(participantContextIDNamespace, []byte(orchestrationID+"/"+activityID))
	return strings.ReplaceAll(id.String(), "-", "")
}

// decodeJWTClaims base64-decodes the payload section of a JWT and returns it as a raw JSON string.
func decodeJWTClaims(jwtToken string) (map[string]any, error) {
	parts := strings.SplitN(jwtToken, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("error decoding JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("error parsing JWT payload: %w", err)
	}
	return claims, nil
}
