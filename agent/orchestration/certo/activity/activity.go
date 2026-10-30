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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/token"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type CertoActivityProcessor struct {
	api.BaseActivityProcessor
	Monitor       system.LogMonitor
	TokenProvider token.TokenProvider
	HttpClient    *http.Client
	CertoURL      string
	tracer        trace.Tracer
}

type certoData struct {
	ParticipantID        string `json:"cfm.participant.id" validate:"required"`
	ParticipantContextId string `json:"participantContextId" validate:"required"`
}

const dataspaceIdentifierKey = "dataspaceIdentifier"

type Config struct {
	system.LogMonitor
	token.TokenProvider
	HttpClient *http.Client
	CertoURL   string
}

func NewProcessor(config *Config) *CertoActivityProcessor {
	return &CertoActivityProcessor{
		Monitor:       config.LogMonitor,
		TokenProvider: config.TokenProvider,
		HttpClient:    config.HttpClient,
		CertoURL:      config.CertoURL,
		tracer:        otel.GetTracerProvider().Tracer("cfm.agent.certo"),
	}
}

func (p CertoActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	_, span := p.tracer.Start(ctx.Context(), "cfm.agent.certo.deploy")
	defer span.End()

	var data certoData
	if err := ctx.ReadValues(&data); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Certo activity for orchestration %s: %w", ctx.OID(), err)}
	}
	span.SetAttributes(attribute.String("cfm.participantContextId", data.ParticipantContextId))

	// Todo: ugly hack, that leeches off of the `cfm.issuer` properties
	properties, err := ctx.VpaProperties(model.IssuerServiceType)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error reading vpa data: %w", err)}
	}

	exchangedToken, err := p.TokenProvider.GetToken(ctx.Context(), "certo-mgmt-api:write", "sudo")
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error getting token: %w", err)}
	}

	body := struct {
		ParticipantContextId string `json:"participantContextId"`
		Did                  string `json:"did"`
		DataspaceIdentifier  string `json:"dataspaceIdentifier,omitempty"`
	}{
		ParticipantContextId: data.ParticipantContextId,
		Did:                  data.ParticipantID,
		DataspaceIdentifier:  dataspaceIdentifier(properties),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error marshalling body: %w", err)}
	}
	// --- invoke POST endpoint on Certo's API ----
	rq, err := http.NewRequestWithContext(ctx.Context(), "POST", p.CertoURL+"/management/v1/participant-contexts", strings.NewReader(string(jsonBody)))
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error creating request: %w", err)}
	}
	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Authorization", "Bearer "+exchangedToken)
	response, err := p.HttpClient.Do(rq)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error executing request: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		errorbody, _ := io.ReadAll(response.Body)
		e := fmt.Errorf("failed to create participant context on Certo: received status code %d, body: %s", response.StatusCode, string(errorbody))
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: e}
	}

	p.Monitor.Infof("Certo activity for participant '%s' (participant context = %s) completed successfully", data.ParticipantID, data.ParticipantContextId)
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p CertoActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {
	_, span := p.tracer.Start(ctx.Context(), "cfm.agent.certo.dispose")
	defer span.End()

	var data certoData
	if err := ctx.ReadValues(&data); err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Certo activity for orchestration %s: %w", ctx.OID(), err)}
	}
	span.SetAttributes(attribute.String("cfm.participantContextId", data.ParticipantContextId))

	exchangedToken, err := p.TokenProvider.GetToken(ctx.Context(), "certo-mgmt-api:write", "sudo")
	if err != nil {
		span.RecordError(err)
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error getting token: %w", err)}
	}

	// --- invoke DELETE endpoint on Certo's API ----
	rq, err := http.NewRequestWithContext(ctx.Context(), http.MethodDelete, p.CertoURL+"/management/v1/participant-contexts/"+url.PathEscape(data.ParticipantContextId), nil)
	if err != nil {
		span.RecordError(err)
		p.Monitor.Warnf("error creating request: %v", err)
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}
	rq.Header.Set("Content-Type", "application/json")
	rq.Header.Set("Authorization", "Bearer "+exchangedToken)
	response, err := p.HttpClient.Do(rq)
	if err != nil {
		p.Monitor.Warnf("error executing request: %v", err)
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		p.Monitor.Infof("Certo participant context '%s' does not exist, nothing to dispose", data.ParticipantContextId)
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}
	if response.StatusCode != http.StatusNoContent {
		errorBody, _ := io.ReadAll(response.Body)
		p.Monitor.Warnf("failed to delete participant context on Certo: received status code %d, body: %s", response.StatusCode, string(errorBody))
		return api.ActivityResult{Result: api.ActivityResultComplete}
	}

	p.Monitor.Infof("Certo dispose for participant context '%s' completed", data.ParticipantContextId)
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

// dataspaceIdentifier returns the participant's dataspace-assigned identifier from the VPA properties, or an empty
// string if it is not set, since the identifier is optional in Certo.
func dataspaceIdentifier(properties map[string]any) string {
	v, _ := properties[dataspaceIdentifierKey].(string)
	return v
}
