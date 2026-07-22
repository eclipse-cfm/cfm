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
// plane. On dispose it reverses both operations.
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

// dataPlaneIDSuffix is appended to the participant context id to derive the data-plane instance id.
const dataPlaneIDSuffix = "-siglet"

// defaultTokenSource is applied to a transfer-type mapping whose tokenSource is left unset.
const defaultTokenSource = "provider"

// dataflowsPathTemplate is the Siglet DPS signaling path the control plane sends flow events to. The
// single verb is the participant context id. It is appended to the configured Siglet signaling URL.
const dataflowsPathTemplate = "/api/v1/%s/dataflows"

type Config struct {
	system.LogMonitor
	// SigletSignalingURL is the base URL of the Siglet signaling API (scheme://host:port). The
	// per-participant DPS endpoint registered with the control plane is derived from it.
	SigletSignalingURL        string
	TransferTypeMappingClient siglet.TransferTypeMappingClient
	DataPlaneClient           controlplane.DataPlaneRegistrationClient
}

type SigletActivityProcessor struct {
	api.BaseActivityProcessor
	monitor                   system.LogMonitor
	sigletSignalingURL        string
	transferTypeMappingClient siglet.TransferTypeMappingClient
	dataPlaneClient           controlplane.DataPlaneRegistrationClient
	tracer                    trace.Tracer
}

func NewProcessor(config *Config) *SigletActivityProcessor {
	return &SigletActivityProcessor{
		monitor:                   config.LogMonitor,
		sigletSignalingURL:        config.SigletSignalingURL,
		transferTypeMappingClient: config.TransferTypeMappingClient,
		dataPlaneClient:           config.DataPlaneClient,
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

	var data sigletData
	if err := ctx.ReadValues(&data); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Siglet activity for orchestration %s: %w", ctx.OID(), err)}
	}
	participantContextId := data.ParticipantContextId
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextId))

	return p.handleDeployAction(spanCtx, participantContextId, mappings)
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
func (p SigletActivityProcessor) handleDeployAction(ctx context.Context, participantContextId string, mappings map[string]siglet.TransferType) api.ActivityResult {
	mapping := siglet.TransferTypeMapping{
		ParticipantContextID: participantContextId,
		Mappings:             mappings,
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

	if err := p.dataPlaneClient.RegisterDataPlane(ctx, participantContextId, p.dataPlaneRegistration(participantContextId, mappings)); err != nil {
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

// dataPlaneRegistration builds the control-plane registration for the Siglet data plane. The transfer
// types are derived from the configured mappings and the endpoint is the participant-scoped Siglet
// DPS signaling endpoint.
func (p SigletActivityProcessor) dataPlaneRegistration(participantContextId string, mappings map[string]siglet.TransferType) controlplane.DataPlaneRegistration {
	transferTypes := make([]string, 0, len(mappings))
	for transferType := range mappings {
		transferTypes = append(transferTypes, transferType)
	}
	return controlplane.DataPlaneRegistration{
		ID:            dataPlaneID(participantContextId),
		TransferTypes: transferTypes,
		Endpoint:      p.dataPlaneEndpoint(participantContextId),
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
