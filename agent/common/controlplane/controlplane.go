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
	"sort"
	"strings"
	"time"

	vault "github.com/eclipse-cfm/cfm/agent/common/vault"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/token"
)

const (
	CreateParticipantURL                                       = "/v5/participants"
	applicationJSON                                            = "application/json"
	ParticipantContextStateCreated     ParticipantContextState = "CREATED"
	ParticipantContextStateActivated   ParticipantContextState = "ACTIVATED"
	ParticipantContextStateDeactivated ParticipantContextState = "DEACTIVATED"
	contextConnector                                           = "https://w3id.org/edc/connector/management/v2"
	dataspaceProfileType                                       = "AssociateDataspaceProfile"
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
	// PatchConfig merges the given entries into the participant context config and VERIFIES they
	// are visible afterwards, re-patching if not (see HttpManagementAPIClient.PatchConfig).
	PatchConfig(ctx context.Context, participantContextID string, config ParticipantContextConfig) error
	// GetConfig reads the participant context config back from the control plane. Values of
	// private entries are returned as stored (encrypted), so only their keys are meaningful to
	// callers.
	GetConfig(ctx context.Context, participantContextID string) (ParticipantContextConfig, error)
	DeleteConfig(ctx context.Context, participantContextID string) error
	DeleteParticipantContext(ctx context.Context, participantContextID string) error
	// AssociateProfiles associates the given dataspace profiles with the participant context in the
	// control plane. Callers are responsible for not invoking it with an empty profiles slice.
	AssociateProfiles(ctx context.Context, participantContextID string, profiles []string) error
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
	// Authorization is the optional DPS authorization profile the control plane uses to authorize
	// signaling exchanges with this data plane. It is a flat object: "type" plus the properties of
	// that profile. Omitted from the request when nil.
	Authorization map[string]any `json:"authorization,omitempty"`
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
	// Monitor, when set, receives a warning for every patch-verification retry — the signal that
	// a concurrent config write was lost and healed. Optional; nil disables the logging only.
	Monitor system.LogMonitor
}

// patchConfigBackoffs are the waits between patch-verification attempts (attempts = len+1). A
// package variable so tests can shrink them.
var patchConfigBackoffs = []time.Duration{100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond, time.Second}

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

// PatchConfig merges the given entries into the participant context config and verifies with a
// read-back that every entry is actually visible, re-patching (bounded, with backoff) when it is
// not. The verification exists because several agents patch the SAME config concurrently during
// provisioning (the edcv agent writes identity/vault entries, the key-management agent writes the
// STS signature entries within milliseconds of each other), and the control plane's merge has
// been observed to lose one side's entries under concurrency — leaving the participant context
// permanently broken (e.g. "No setting found for key edc.iam.sts.oauth.token.url" on every DSP
// dispatch) while both writers had reported success. Each writer verifying and re-merging its OWN
// entries converges to the union regardless of which write was lost. Public entry values are
// compared verbatim; private entries are verified by key presence only (the control plane stores
// and returns them encrypted).
func (h HttpManagementAPIClient) PatchConfig(ctx context.Context, participantContextID string, config ParticipantContextConfig) error {
	var missing []string
	for attempt := 1; ; attempt++ {
		if err := h.patchConfigOnce(ctx, participantContextID, config); err != nil {
			return err
		}
		applied, err := h.GetConfig(ctx, participantContextID)
		if err != nil {
			missing = []string{fmt.Sprintf("(verification read failed: %s)", err)}
		} else if missing = missingConfigKeys(config, applied); len(missing) == 0 {
			return nil
		}
		if attempt > len(patchConfigBackoffs) {
			return fmt.Errorf("participant config patch for '%s' is not visible after %d attempts — a concurrent config write may keep getting lost; missing or mismatched: %s",
				participantContextID, attempt, strings.Join(missing, ", "))
		}
		if h.Monitor != nil {
			h.Monitor.Warnf("Participant config patch for '%s' is not (fully) visible after attempt %d (missing: %s) — re-patching",
				participantContextID, attempt, strings.Join(missing, ", "))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(patchConfigBackoffs[attempt-1]):
		}
	}
}

func (h HttpManagementAPIClient) patchConfigOnce(ctx context.Context, participantContextID string, config ParticipantContextConfig) error {
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

// GetConfig reads the participant context config via GET /v5/participants/{id}/config. The
// response is the management API's JSON-LD rendering; the entry maps are extracted tolerantly
// (plain terms or expanded IRIs, with or without a JSON-literal "@value" wrapper) so this does
// not depend on the server's compaction behavior.
func (h HttpManagementAPIClient) GetConfig(ctx context.Context, participantContextID string) (ParticipantContextConfig, error) {
	accessToken, err := h.TokenProvider.GetToken(ctx, ScopeApiAdmin, participantContextID)
	if err != nil {
		return ParticipantContextConfig{}, fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s%s/%s/config", h.BaseURL, CreateParticipantURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ParticipantContextConfig{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return ParticipantContextConfig{}, fmt.Errorf("failed to read participant context config from control plane: %w", err)
	}
	defer h.closeResponse(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ParticipantContextConfig{}, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return ParticipantContextConfig{}, fmt.Errorf("failed to read participant context config from control plane: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return ParticipantContextConfig{}, fmt.Errorf("failed to parse participant context config response: %w", err)
	}
	return ParticipantContextConfig{
		ParticipantContextID: participantContextID,
		Entries:              extractConfigEntries(doc, "entries"),
		SecretEntries:        extractConfigEntries(doc, "privateEntries"),
	}, nil
}

// missingConfigKeys lists the entries of want that got does not carry: public entries must match
// key AND value, private entries only the key (their values come back encrypted). Sorted for
// stable messages.
func missingConfigKeys(want ParticipantContextConfig, got ParticipantContextConfig) []string {
	var missing []string
	for key, value := range want.Entries {
		if got.Entries[key] != value {
			missing = append(missing, "entries["+key+"]")
		}
	}
	for key := range want.SecretEntries {
		if _, ok := got.SecretEntries[key]; !ok {
			missing = append(missing, "privateEntries["+key+"]")
		}
	}
	sort.Strings(missing)
	return missing
}

// extractConfigEntries pulls a string map out of a JSON-LD-ish document: the property is matched
// by its local name (plain term, or IRI suffix after '/' or '#'), and a JSON-literal wrapper
// ({"@value": {...}}) is unwrapped when present.
func extractConfigEntries(doc map[string]any, term string) map[string]string {
	entries := map[string]string{}
	for key, value := range doc {
		if localName(key) != term {
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if wrapped, ok := object["@value"].(map[string]any); ok {
			object = wrapped
		}
		for entryKey, entryValue := range object {
			if s, ok := entryValue.(string); ok {
				entries[entryKey] = s
			} else {
				entries[entryKey] = fmt.Sprintf("%v", entryValue)
			}
		}
	}
	return entries
}

func localName(iriOrTerm string) string {
	if idx := strings.LastIndexAny(iriOrTerm, "/#"); idx >= 0 {
		return iriOrTerm[idx+1:]
	}
	return iriOrTerm
}

// RegisterDataPlane registers a data-plane instance with the control plane for the given participant
// context via PUT /v5/participants/{participantContextID}/dataplanes.
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
// DELETE /v5/participants/{participantContextID}/dataplanes/{dataPlaneID}.
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

// AssociateProfiles associates the given dataspace profiles with the participant context via
// PUT /v5/participants/{participantContextID}/profiles.
func (h HttpManagementAPIClient) AssociateProfiles(ctx context.Context, participantContextID string, profiles []string) error {
	accessToken, err := h.TokenProvider.GetToken(ctx, ScopeApiAdmin, participantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	jsonLdData := map[string]any{
		"@context": []string{contextConnector},
		"@type":    dataspaceProfileType,
		"profiles": profiles,
	}

	payload, err := json.Marshal(jsonLdData)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s%s/%s/profiles", h.BaseURL, CreateParticipantURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", applicationJSON)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to associate dataspace profiles on control plane: %w", err)
	}

	defer h.closeResponse(resp)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to associate dataspace profiles on control plane: received status code %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (h HttpManagementAPIClient) closeResponse(resp *http.Response) {
	func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
}
