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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/orchestration/edcv"
	. "github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// DataspaceProfilesKey is the key in the cfm.connector VPA properties that carries the list of
// dataspace profiles (strings) to associate with the participant context in the control plane.
const DataspaceProfilesKey = "dataspaceProfiles"

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

	// dataspace profiles are optional; they are carried in the cfm.connector VPA properties
	props, err := ctx.VpaProperties(model.ConnectorType)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error reading connector VPA properties for orchestration %s: %w", ctx.OID(), err)}
	}
	profiles, err := extractProfiles(props)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error parsing dataspace profiles for orchestration %s: %w", ctx.OID(), err)}
	}

	participantContextId := data.ParticipantContextId
	span.SetAttributes(attribute.String("cfm.participantContextId", participantContextId))
	return p.handleDeployAction(ctx, data, participantContextId, profiles)
}

func (p EDCVActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	var data edcData
	err := ctx.ReadValues(&data)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing EDC-V activity for orchestration %s: %w", ctx.OID(), err)}
	}
	return p.handleDisposeAction(ctx.Context(), data.ParticipantContextId)
}

// handleDeployAction creates the participant context and config in the EDC control plane and, when
// dataspace profiles are supplied, associates them with the participant context.
func (p EDCVActivityProcessor) handleDeployAction(ctx api.ActivityContext, data edcData, participantContextId string, profiles []string) api.ActivityResult {

	did := data.ParticipantID
	if !strings.HasPrefix(did, "did:web:") {
		p.Monitor.Warnf("Participant identifiers are expected to be Web-DIDs, but this one was not: '%s'. Subsequent communication may be severely impacted!", did)
	}

	// the control plane authenticates to Vault via token exchange (jwtlet), so only the vault
	// config (no credentials) is needed; the participant context id is the token-exchange resource.
	vaultConfig := edcv.VaultConfig{
		VaultURL:   p.VaultURL,
		SecretPath: "v1/participants",
		FolderPath: participantContextId + "/controlplane",
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

	// associate dataspace profiles with the participant context, when supplied via VPA properties
	if len(profiles) > 0 {
		if err := p.ManagementAPIClient.AssociateProfiles(ctx.Context(), participantContextId, profiles); err != nil {
			ctrl.RecordError(err)
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot associate dataspace profiles in control plane for orchestration %s: %w", ctx.OID(), err)}
		}
		ctrl.AddEvent("Associated dataspace profiles in Control Plane")
	} else {
		p.Monitor.Infof("No dataspace profiles found for orchestration %s; skipping profile association", ctx.OID())
	}

	ctrl.End()
	p.Monitor.Infof("EDCV activity for participant '%s' (client ID = %s) completed successfully", data.ParticipantID, data.ParticipantContextId)

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// extractProfiles reads and decodes the dataspace profiles from the connector VPA properties. It
// returns nil (not an error) when the properties or the profiles key are absent or empty.
func extractProfiles(props map[string]any) ([]string, error) {
	if props == nil {
		return nil, nil
	}
	raw, ok := props[DataspaceProfilesKey]
	if !ok || raw == nil {
		return nil, nil
	}
	// round-trip through JSON to decode the untyped property value into a typed string slice
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var profiles []string
	if err := json.Unmarshal(encoded, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
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
