//  Copyright (c) 2026 Metaform Systems, Inc
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

// Package activity implements the deploy/dispose activity for the Siglet data-plane agent. On deploy
// it reads the transfer-type mappings from the cfm.dataplane VPA properties, configures them in
// Siglet for the participant context, and registers the Siglet data-plane instance with the control
// plane, optionally with a DPS authorization profile. On dispose it reverses both operations.
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/common/siglet"
	. "github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TransferTypeMappingsKey is the key in the cfm.dataplane VPA properties that carries the map of
// transfer-type mappings (transferType -> mapping) to configure in Siglet.
const TransferTypeMappingsKey = "transferTypeMappings"

// AuthorizationKey is the key in the cfm.dataplane VPA properties that opts the data-plane
// registration into a DPS authorization profile. Its presence is what enables the profile: an object
// carrying only the type takes every other property from the agent configuration, an absent key
// registers the data plane without any authorization.
const AuthorizationKey = "authorization"

// authorizationType is the DPS authorization profile the agent knows how to populate, and the only
// value accepted for the type property of the VPA authorization object.
const authorizationType = "oauth2_token_exchange"

// dataPlaneIDSuffix is appended to the participant context id to derive the data-plane instance id.
const dataPlaneIDSuffix = "-siglet"

// defaultTokenSource is applied to a transfer-type mapping whose tokenSource is left unset.
const defaultTokenSource = "provider"

// dataflowsPathTemplate is the Siglet DPS signaling path the control plane sends flow events to. The
// single verb is the participant context id. It is appended to the configured Siglet signaling URL.
const dataflowsPathTemplate = "/api/v1/%s/dataflows"

// AuthorizationConfig holds the agent-level fallbacks for the data-plane authorization profile. Each
// value is used for the corresponding property when the cfm.dataplane VPA leaves it unset.
type AuthorizationConfig struct {
	// TokenExchangeEndpoint is the RFC 8693 token exchange endpoint of the broker, from tokenexchange.url.
	TokenExchangeEndpoint string
	// Issuer is the expected iss claim of tokens minted by the broker, from tokenexchange.issuer.
	Issuer string
	// JwksUri is the broker's JWKS endpoint, from tokenexchange.jwksUri.
	JwksUri string
	// Scope is the scope requested for exchanged tokens, from tokenexchange.scope. Optional: when it
	// is empty and the VPA sets none either, the connector falls back to its own configuration.
	Scope string
}

type Config struct {
	system.LogMonitor
	// SigletSignalingURL is the base URL of the Siglet signaling API (scheme://host:port). The
	// per-participant DPS endpoint registered with the control plane is derived from it.
	SigletSignalingURL        string
	TransferTypeMappingClient siglet.TransferTypeMappingClient
	DataPlaneClient           controlplane.DataPlaneRegistrationClient
	// Authorization holds the fallbacks for the data-plane authorization profile.
	Authorization AuthorizationConfig
}

type SigletActivityProcessor struct {
	api.BaseActivityProcessor
	monitor                   system.LogMonitor
	sigletSignalingURL        string
	transferTypeMappingClient siglet.TransferTypeMappingClient
	dataPlaneClient           controlplane.DataPlaneRegistrationClient
	authorization             AuthorizationConfig
	tracer                    trace.Tracer
}

func NewProcessor(config *Config) *SigletActivityProcessor {
	return &SigletActivityProcessor{
		monitor:                   config.LogMonitor,
		sigletSignalingURL:        config.SigletSignalingURL,
		transferTypeMappingClient: config.TransferTypeMappingClient,
		dataPlaneClient:           config.DataPlaneClient,
		authorization:             config.Authorization,
		tracer:                    otel.GetTracerProvider().Tracer("cfm.agent.siglet"),
	}
}

type sigletData struct {
	ParticipantContextId string `json:"participantContextId" validate:"required"`
}

func (p SigletActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	spanCtx, span := p.tracer.Start(ctx.Context(), "cfm.agent.siglet.deploy")
	defer span.End()

	props, err := ctx.VpaProperties(model.DataPlaneType)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error reading data plane VPA properties for orchestration %s: %w", ctx.OID(), err)}
	}

	// If there is no data plane VPA, or it carries no transfer-type mappings, there is nothing to do.
	mappings, err := extractMappings(props)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error parsing transfer type mappings for orchestration %s: %w", ctx.OID(), err)}
	}
	if len(mappings) == 0 {
		p.monitor.Infof("No data plane transfer type mappings found for orchestration %s; nothing to configure", ctx.OID())
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}

	authorization, err := extractAuthorization(props)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error parsing data plane authorization for orchestration %s: %w", ctx.OID(), err)}
	}

	var data sigletData
	if err := ctx.ReadValues(&data); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Siglet activity for orchestration %s: %w", ctx.OID(), err)}
	}
	participantContextId := data.ParticipantContextId
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextId))

	return p.handleDeployAction(spanCtx, participantContextId, mappings, authorization)
}

func (p SigletActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	var data sigletData
	if err := ctx.ReadValues(&data); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Siglet activity for orchestration %s: %w", ctx.OID(), err)}
	}
	return p.handleDisposeAction(ctx.Context(), data.ParticipantContextId)
}

// handleDeployAction configures the transfer-type mappings in Siglet (upsert) and registers the
// Siglet data-plane instance with the control plane.
func (p SigletActivityProcessor) handleDeployAction(ctx context.Context, participantContextId string, mappings map[string]siglet.TransferType, authorization *authorizationProperties) api.ActivityResult {
	// Resolve the authorization profile before touching Siglet, so an incomplete profile fails the
	// activity without leaving a half-applied transfer-type mapping behind.
	profile, err := p.authorizationProfile(participantContextId, authorization)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}

	// Only mappings that carry enough information to be a valid Siglet mapping are configured in
	// Siglet; the rest are still registered as data-plane transfer types below.
	sigletMappings := make(map[string]siglet.TransferType, len(mappings))
	for name, tt := range mappings {
		if isSigletConfigurable(tt) {
			sigletMappings[name] = tt
		}
	}

	if len(sigletMappings) > 0 {
		mapping := siglet.TransferTypeMapping{
			ParticipantContextID: participantContextId,
			Mappings:             sigletMappings,
		}

		// upsert: replace when a mapping already exists, otherwise create
		existing, err := p.transferTypeMappingClient.GetTransferTypeMapping(ctx, participantContextId)
		if err != nil {
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot read transfer type mapping from Siglet: %w", err)}
		}
		if existing != nil {
			if err := p.transferTypeMappingClient.ReplaceTransferTypeMapping(ctx, mapping); err != nil {
				return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot replace transfer type mapping in Siglet: %w", err)}
			}
		} else {
			if err := p.transferTypeMappingClient.CreateTransferTypeMapping(ctx, mapping); err != nil {
				return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot create transfer type mapping in Siglet: %w", err)}
			}
		}
	} else {
		p.monitor.Infof("No Siglet-configurable transfer type mappings for participant '%s'; registering data plane only", participantContextId)
	}

	if err := p.dataPlaneClient.RegisterDataPlane(ctx, participantContextId, p.dataPlaneRegistration(participantContextId, mappings, profile)); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot register data plane in control plane: %w", err)}
	}

	p.monitor.Infof("Siglet activity for participant '%s' completed successfully", participantContextId)
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// handleDisposeAction removes the transfer-type mappings from Siglet and unregisters the data-plane
// instance from the control plane. Errors are logged but not propagated, so a failure does not block
// rollback of sibling agents.
func (p SigletActivityProcessor) handleDisposeAction(ctx context.Context, participantContextId string) api.ActivityResult {
	var errors []error

	if err := p.transferTypeMappingClient.DeleteTransferTypeMapping(ctx, participantContextId); err != nil {
		errors = append(errors, err)
	}
	if err := p.dataPlaneClient.UnregisterDataPlane(ctx, participantContextId, dataPlaneID(participantContextId)); err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		errStrings := Collect(Map(From(errors), func(err error) string { return err.Error() }))
		p.monitor.Warnf("one or more errors occurred while disposing Siglet data plane for '%s': [%s]", participantContextId, strings.Join(errStrings, ", "))
	}
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// extractMappings reads and decodes the transfer-type mappings from the data plane VPA properties.
// It returns an empty map (not an error) when the properties or the mappings key are absent.
func extractMappings(props map[string]any) (map[string]siglet.TransferType, error) {
	if props == nil {
		return nil, nil
	}
	raw, ok := props[TransferTypeMappingsKey]
	if !ok || raw == nil {
		return nil, nil
	}
	// round-trip through JSON to decode the untyped property bag into typed mappings
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var mappings map[string]siglet.TransferType
	if err := json.Unmarshal(encoded, &mappings); err != nil {
		return nil, err
	}
	// tokenSource is optional on input; default it to "provider" when unset
	for name, mapping := range mappings {
		if mapping.TokenSource == "" {
			mapping.TokenSource = defaultTokenSource
			mappings[name] = mapping
		}
	}
	return mappings, nil
}

// authorizationProperties is the typed view of the authorization object in the cfm.dataplane VPA
// properties. Type is required and selects the profile; every other property is optional on input:
// what is not set falls back to the agent configuration, except resource which falls back to the
// participant context id.
type authorizationProperties struct {
	Type                  string `json:"type"`
	TokenExchangeEndpoint string `json:"tokenExchangeEndpoint"`
	Issuer                string `json:"issuer"`
	JwksUri               string `json:"jwksUri"`
	Resource              string `json:"resource"`
	Scope                 string `json:"scope"`
}

// extractAuthorization reads and decodes the authorization object from the data plane VPA properties.
// It returns nil (not an error) when the properties or the authorization key are absent, which is
// what tells the caller to register the data plane without an authorization profile.
func extractAuthorization(props map[string]any) (*authorizationProperties, error) {
	if props == nil {
		return nil, nil
	}
	raw, ok := props[AuthorizationKey]
	if !ok || raw == nil {
		return nil, nil
	}
	// round-trip through JSON to decode the untyped property bag into typed properties
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var authorization authorizationProperties
	if err := json.Unmarshal(encoded, &authorization); err != nil {
		return nil, err
	}
	return &authorization, nil
}

// authorizationProfile resolves the DPS authorization profile registered with the data plane. The
// profile is built when the VPA authorization object declares the supported type; each property is
// then taken from the VPA when set and from the agent configuration otherwise, except resource, which
// defaults to the participant context id. It returns nil when the VPA carries no authorization
// object, and an error when the declared type is unsupported or a required property resolves to empty
// - registering a half-populated profile would only surface later as opaque signaling failures.
//
// Note on resource: the control plane derives the caller identity of an incoming signaling request
// from the token's client_id claim, falling back to sub (the resource URI), and matches it against
// the registered dataplaneId, which is "<participantContextId>-siglet". With the default resource the
// broker must therefore mint client_id = "<participantContextId>-siglet"; deployments whose broker
// does not mint client_id should set resource explicitly in the VPA.
func (p SigletActivityProcessor) authorizationProfile(participantContextId string, vpa *authorizationProperties) (map[string]any, error) {
	if vpa == nil {
		return nil, nil
	}

	if profileType := strings.TrimSpace(vpa.Type); profileType != authorizationType {
		if profileType == "" {
			return nil, fmt.Errorf("data plane authorization property 'type' is not set in the %s VPA: the only supported type is '%s'", model.DataPlaneType, authorizationType)
		}
		return nil, fmt.Errorf("unsupported data plane authorization type '%s' in the %s VPA: the only supported type is '%s'", profileType, model.DataPlaneType, authorizationType)
	}

	endpoint := firstNonEmpty(vpa.TokenExchangeEndpoint, p.authorization.TokenExchangeEndpoint)
	issuer := firstNonEmpty(vpa.Issuer, p.authorization.Issuer)
	jwksUri := firstNonEmpty(vpa.JwksUri, p.authorization.JwksUri)
	resource := firstNonEmpty(vpa.Resource, participantContextId)
	scope := firstNonEmpty(vpa.Scope, p.authorization.Scope)

	required := []struct {
		property  string
		value     string
		configKey string
	}{
		{"tokenExchangeEndpoint", endpoint, "tokenexchange.url"},
		{"issuer", issuer, "tokenexchange.issuer"},
		{"jwksUri", jwksUri, "tokenexchange.jwksUri"},
		{"resource", resource, ""},
	}
	for _, r := range required {
		if r.value == "" {
			if r.configKey == "" {
				return nil, fmt.Errorf("data plane authorization property '%s' is not set in the %s VPA", r.property, model.DataPlaneType)
			}
			return nil, fmt.Errorf("data plane authorization property '%s' is not set in the %s VPA and no fallback is configured (%s)", r.property, model.DataPlaneType, r.configKey)
		}
	}

	profile := map[string]any{
		"type":                  authorizationType,
		"tokenExchangeEndpoint": endpoint,
		"issuer":                issuer,
		"jwksUri":               jwksUri,
		"resource":              resource,
	}
	// scope is optional end to end: when neither the VPA nor the agent sets it, the connector falls
	// back to its own configured default scope.
	if scope != "" {
		profile["scope"] = scope
	}
	return profile, nil
}

// firstNonEmpty returns the first of the given values that is not blank, trimmed, or "" if all are.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// isSigletConfigurable reports whether a transfer-type mapping carries enough information to be
// configured in Siglet: an endpoint type plus either a static endpoint or at least one endpoint
// mapping. A mapping that only names its transfer type is still registered as a data-plane transfer
// type with the control plane, but is not sent to Siglet.
func isSigletConfigurable(tt siglet.TransferType) bool {
	if strings.TrimSpace(tt.EndpointType) == "" {
		return false
	}
	return strings.TrimSpace(tt.Endpoint) != "" || len(tt.EndpointMappings) > 0
}

// dataPlaneRegistration builds the control-plane registration for the Siglet data plane. The transfer
// types are derived from the configured mappings and the endpoint is the participant-scoped Siglet
// DPS signaling endpoint. The authorization profile is nil unless the VPA asked for one.
func (p SigletActivityProcessor) dataPlaneRegistration(participantContextId string, mappings map[string]siglet.TransferType, authorization map[string]any) controlplane.DataPlaneRegistration {
	transferTypes := make([]string, 0, len(mappings))
	for transferType := range mappings {
		transferTypes = append(transferTypes, transferType)
	}
	return controlplane.DataPlaneRegistration{
		ID:            dataPlaneID(participantContextId),
		TransferTypes: transferTypes,
		Endpoint:      p.dataPlaneEndpoint(participantContextId),
		Authorization: authorization,
	}
}

// dataPlaneEndpoint builds the participant-scoped Siglet DPS signaling endpoint the control plane
// sends flow events to, e.g. http://siglet...:8081/api/v1/<participantContextId>/dataflows.
func (p SigletActivityProcessor) dataPlaneEndpoint(participantContextId string) string {
	return strings.TrimRight(p.sigletSignalingURL, "/") + fmt.Sprintf(dataflowsPathTemplate, participantContextId)
}

func dataPlaneID(participantContextId string) string {
	return participantContextId + dataPlaneIDSuffix
}
