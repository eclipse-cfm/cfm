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

package siglet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/eclipse-cfm/cfm/common/token"
)

// EndpointMapping resolves a data endpoint dynamically from flow metadata (see Siglet's
// endpoint_mappings). Used as an alternative to a static Endpoint on a TransferType.
type EndpointMapping struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Endpoint string `json:"endpoint"`
}

// TransferType describes how Siglet maps a single DPS transfer type (the flow profile) to a data
// endpoint and a token source. It mirrors a Siglet `[[transfer_types]]` block.
type TransferType struct {
	// TransferType is the flow profile, e.g. "HttpData-PULL".
	TransferType string `json:"transferType"`
	// EndpointType is the backend type, e.g. "HTTP".
	EndpointType string `json:"endpointType"`
	// TokenSource is either "provider" or "client". Optional on input; defaults to "provider".
	TokenSource string `json:"tokenSource"`
	// Endpoint is the static data endpoint. Use this OR EndpointMappings, not both.
	Endpoint string `json:"endpoint,omitempty"`
	// EndpointMappings resolves the endpoint dynamically from flow metadata.
	EndpointMappings []EndpointMapping `json:"endpointMappings,omitempty"`
	// TxRenewalSupport indicates whether the transfer type supports transfer renewal. Defaults to false.
	TxRenewalSupport bool `json:"txRenewalSupport"`
}

// TransferTypeMapping is the complete set of transfer-type mappings for a single participant context.
// Configuring a context replaces its whole map (there is no per-entry patch).
type TransferTypeMapping struct {
	ParticipantContextID string                  `json:"participantContextId"`
	Mappings             map[string]TransferType `json:"mappings"`
}

// TransferTypeMappingClient interacts with Siglet's Management API to CRUD per-participant-context
// transfer-type mappings (the `/transfer-type-mappings` resource). See the Siglet runtime docs.
type TransferTypeMappingClient interface {
	CreateTransferTypeMapping(ctx context.Context, mapping TransferTypeMapping) error
	GetTransferTypeMapping(ctx context.Context, participantContextID string) (*TransferTypeMapping, error)
	ReplaceTransferTypeMapping(ctx context.Context, mapping TransferTypeMapping) error
	DeleteTransferTypeMapping(ctx context.Context, participantContextID string) error
}

// NewTransferTypeMappingClient constructs a client for Siglet's transfer-type-mapping Management API.
func NewTransferTypeMappingClient(httpClient *http.Client, tokenProvider token.TokenProvider, url string) TransferTypeMappingClient {
	return &httpManagementApiClient{
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
		BaseURL:       url,
	}
}

func (s httpManagementApiClient) CreateTransferTypeMapping(ctx context.Context, mapping TransferTypeMapping) error {
	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiWrite, mapping.ParticipantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	payload, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal transfer type mapping: %w", err)
	}

	url := fmt.Sprintf("%s/transfer-type-mappings", s.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create Siglet transfer type mapping: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create Siglet transfer type mapping: received status code %d", resp.StatusCode)
	}
	return nil
}

func (s httpManagementApiClient) GetTransferTypeMapping(ctx context.Context, participantContextID string) (*TransferTypeMapping, error) {
	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiRead, participantContextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/transfer-type-mappings/%s", s.BaseURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Siglet transfer type mapping request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get Siglet transfer type mapping: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get Siglet transfer type mapping: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var mapping TransferTypeMapping
	if err := json.Unmarshal(body, &mapping); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transfer type mapping: %w", err)
	}
	return &mapping, nil
}

func (s httpManagementApiClient) ReplaceTransferTypeMapping(ctx context.Context, mapping TransferTypeMapping) error {
	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiWrite, mapping.ParticipantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	payload, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal transfer type mapping: %w", err)
	}

	url := fmt.Sprintf("%s/transfer-type-mappings/%s", s.BaseURL, mapping.ParticipantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to replace Siglet transfer type mapping: %w", err)
	}
	defer drainAndClose(resp)

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to replace Siglet transfer type mapping: received status code %d", resp.StatusCode)
	}
	return nil
}

func (s httpManagementApiClient) DeleteTransferTypeMapping(ctx context.Context, participantContextID string) error {
	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiWrite, participantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/transfer-type-mappings/%s", s.BaseURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create Siglet transfer type mapping request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete Siglet transfer type mapping: %w", err)
	}
	defer drainAndClose(resp)

	// treat an already-absent mapping as success so dispose is idempotent
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error deleting Siglet transfer type mapping: received status code %d", resp.StatusCode)
	}
	return nil
}

// drainAndClose drains and closes the response body to avoid connection/resource leaks.
func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
