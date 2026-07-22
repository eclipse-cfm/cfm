/*
 *  Copyright (c) 2025 Metaform Systems, Inc.
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

package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	vault "github.com/eclipse-cfm/cfm/agent/common/vault"
	"github.com/eclipse-cfm/cfm/common/token"
)

const (
	CreateParticipantURL                                       = "/v5beta/participants"
	applicationJSON                                            = "application/json"
	ParticipantContextStateCreated     ParticipantContextState = "CREATED"
	ParticipantContextStateActivated   ParticipantContextState = "ACTIVATED"
	ParticipantContextStateDeactivated ParticipantContextState = "DEACTIVATED"
	contextConnector                                           = "https://w3id.org/edc/connector/management/v2"
	ScopeApiWrite                                              = "management-api:write"
	ScopeApiRead                                               = "management-api:read"
	// ScopeApiAdmin is required for participant context lifecycle operations: they act on
	// participant contexts other than the token's own subject, which only admin may do.
	ScopeApiAdmin = "management-api:admin"
)

type ParticipantContextConfig struct {
	ParticipantContextID string            `json:"participantContextId"`
	Entries              map[string]string `json:"entries"`
	SecretEntries        map[string]string `json:"privateEntries"`
}

func NewParticipantContextConfig(participantContextID string, participantID string, vConfig vault.Config) ParticipantContextConfig {
	vaultConfig := map[string]any{
		"config": vConfig,
	}
	return ParticipantContextConfig{
		ParticipantContextID: participantContextID,
		Entries: map[string]string{
			"edc.iam.issuer.id":  participantID,
			"edc.participant.id": participantID,
		},
		SecretEntries: map[string]string{
			"edc.vault.hashicorp.config": serialize(vaultConfig),
		},
	}
}

func serialize(object any) string {
	res, _ := json.Marshal(object)
	return string(res)
}

type ParticipantContextState string

type ParticipantContext struct {
	ParticipantContextID string                  `json:"id"`
	Identifier           string                  `json:"identity"`
	Properties           map[string]any          `json:"properties"`
	State                ParticipantContextState `json:"state"`
}

type ManagementAPIClient interface {
	CreateParticipantContext(ctx context.Context, manifest ParticipantContext) error
	PatchConfig(ctx context.Context, participantContextID string, config ParticipantContextConfig) error
	DeleteConfig(ctx context.Context, participantContextID string) error
	DeleteParticipantContext(ctx context.Context, participantContextID string) error
}

// DataPlaneRegistration describes a data-plane instance to register with the control plane for a
// participant context. For a Siglet data plane, Endpoint is the DPS signaling endpoint and the
// transfer types are the ones configured as transfer-type mappings in Siglet.
type DataPlaneRegistration struct {
	// ID is the unique identifier of the data-plane instance, e.g. "<participant>-siglet".
	ID string `json:"dataplaneId"`
	// TransferTypes are the transfer types the data plane supports, e.g. "HttpData-PULL".
	TransferTypes []string `json:"transferTypes"`
	// Endpoint is the data plane's DPS signaling endpoint the control plane sends flow events to.
	Endpoint string `json:"endpoint"`
}

// DataPlaneRegistrationClient registers and unregisters data-plane instances with the EDC control
// plane, scoped to a participant context. HttpManagementAPIClient implements this interface.
type DataPlaneRegistrationClient interface {
	RegisterDataPlane(ctx context.Context, participantContextID string, registration DataPlaneRegistration) error
	UnregisterDataPlane(ctx context.Context, participantContextID string, dataPlaneID string) error
}

type HttpManagementAPIClient struct {
	BaseURL       string
	TokenProvider token.TokenProvider
	HttpClient    *http.Client
}

func (h HttpManagementAPIClient) DeleteConfig(ctx context.Context, participantContextID string) error {
	// fixme: there is no dedicated delete endpoint
	return nil
}

func (h HttpManagementAPIClient) DeleteParticipantContext(ctx context.Context, participantContextID string) error {
	accessToken, err := h.TokenProvider.GetToken(ctx, ScopeApiAdmin, participantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s%s/%s", h.BaseURL, CreateParticipantURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	h.closeResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to delete participant context on control plane: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("participant context %s not found in control plane", participantContextID)
	case http.StatusOK:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete participant context on control plane: received status code %d, body: %s", resp.StatusCode, string(body))
	}
}

func (h HttpManagementAPIClient) CreateParticipantContext(ctx context.Context, manifest ParticipantContext) error {
	accessToken, err := h.TokenProvider.GetToken(ctx, ScopeApiAdmin, manifest.ParticipantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	jsonLdData := map[string]any{
		"@context":   []string{contextConnector},
		"@type":      "ParticipantContext",
		"@id":        manifest.ParticipantContextID,
		"identity":   manifest.Identifier,
		"properties": manifest.Properties,
		"state":      manifest.State,
	}

	payload, err := json.Marshal(jsonLdData)
	if err != nil {
		return err
	}

	url := h.BaseURL + CreateParticipantURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", applicationJSON)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create participant context on control plane: %w", err)
	}

	h.closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create participant context on control plane: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (h HttpManagementAPIClient) PatchConfig(ctx context.Context, participantContextID string, config ParticipantContextConfig) error {
	accessToken, err := h.TokenProvider.GetToken(ctx, ScopeApiAdmin, participantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	configData := map[string]any{
		"@context":       []string{contextConnector},
		"@type":          "ParticipantContextConfig",
		"entries":        config.Entries,
		"privateEntries": config.SecretEntries,
		"identity":       config.ParticipantContextID,
	}

	payload, err := json.Marshal(configData)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s%s/%s/config", h.BaseURL, CreateParticipantURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", applicationJSON)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to patch participant context config on control plane: %w", err)
	}

	defer h.closeResponse(resp)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to patch participant context config on control plane: received status code %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// RegisterDataPlane registers a data-plane instance with the control plane for the given participant
// context via PUT /v5beta/participants/{participantContextID}/dataplanes.
func (h HttpManagementAPIClient) RegisterDataPlane(ctx context.Context, participantContextID string, registration DataPlaneRegistration) error {
	accessToken, err := h.TokenProvider.GetToken(ctx, ScopeApiAdmin, participantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	payload, err := json.Marshal(registration)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s%s/%s/dataplanes", h.BaseURL, CreateParticipantURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", applicationJSON)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register data plane on control plane: %w", err)
	}

	defer h.closeResponse(resp)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to register data plane on control plane: received status code %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UnregisterDataPlane removes a previously registered data-plane instance from the control plane via
// DELETE /v5beta/participants/{participantContextID}/dataplanes/{dataPlaneID}.
func (h HttpManagementAPIClient) UnregisterDataPlane(ctx context.Context, participantContextID string, dataPlaneID string) error {
	accessToken, err := h.TokenProvider.GetToken(ctx, ScopeApiAdmin, participantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s%s/%s/dataplanes/%s", h.BaseURL, CreateParticipantURL, participantContextID, dataPlaneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to unregister data plane on control plane: %w", err)
	}

	defer h.closeResponse(resp)

	switch {
	case resp.StatusCode == http.StatusNotFound:
		// treat an already-absent data plane as success so dispose is idempotent
		return nil
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to unregister data plane on control plane: received status code %d, body: %s", resp.StatusCode, string(body))
	}
}

func (h HttpManagementAPIClient) closeResponse(resp *http.Response) {
	func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
}
