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

const (
	ScopeApiRead  = "siglet-read"
	ScopeApiWrite = "siglet-write"
)

type KeyMapping struct {

	// ParticipantContextID is the unique identifier for the context of a participant in a key mapping configuration.
	ParticipantContextID string `json:"participantContextId"`
	// KeyName the name by which the key is referenced by Siglet. Typically, this is used as "keyName" when creating keys in Vault's Transit Engine
	KeyName string `json:"keyName"`
	// KeyID is the value written verbatim into the JWT header so verifiers can locate the key.
	KeyID string `json:"kid"`
}

// ManagementAPIClient is an interface for interacting with Siglet's Management API to CRUD "key-mappings". A KeyMapping
// is a structure that associates a participant context ID with a key name and key ID. The KeyName is sometimes also called
// "privateKeyAlias" in EDC and is used by Vault Transit Engine to identify keys. The KeyID is used to reference the key in
// publicly available key material, such as DID documents and JWKS structures. It will be used to reference the key in JWT headers as
// the "kid" value.
type ManagementAPIClient interface {
	CreateKeyMapping(context.Context, KeyMapping) (err error)
	UpdateKeyMapping(context.Context, KeyMapping) error
	GetKeyMapping(ctx context.Context, participantContextID string) (*KeyMapping, error)
	DeleteKeyMapping(ctx context.Context, participantContextID string) error
}

func NewSigletAPIClient(httpClient *http.Client, tokenProvider token.TokenProvider, url string) ManagementAPIClient {
	return &httpManagementApiClient{
		httpClient:    httpClient,
		tokenProvider: tokenProvider,
		BaseURL:       url,
	}
}

type httpManagementApiClient struct {
	httpClient    *http.Client
	tokenProvider token.TokenProvider
	BaseURL       string
}

func (s httpManagementApiClient) CreateKeyMapping(ctx context.Context, mapping KeyMapping) error {
	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiWrite, mapping.ParticipantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	payload, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal key mapping: %w", err)
	}

	url := fmt.Sprintf("%s/key-mappings", s.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	defer func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if err != nil {
		return fmt.Errorf("failed to create Siglet Mapping: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create Siglet Mapping: received status code %d", resp.StatusCode)
	}

	return nil

}

func (s httpManagementApiClient) UpdateKeyMapping(ctx context.Context, mapping KeyMapping) error {
	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiWrite, mapping.ParticipantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	payload, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal key mapping: %w", err)
	}

	url := fmt.Sprintf("%s/key-mappings/%s", s.BaseURL, mapping.ParticipantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	defer func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if err != nil {
		return fmt.Errorf("failed to create Siglet Mapping: %w", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to create Siglet Mapping: received status code %d", resp.StatusCode)
	}

	return nil
}

func (s httpManagementApiClient) GetKeyMapping(ctx context.Context, participantContextID string) (*KeyMapping, error) {
	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiRead, participantContextID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API access token: %w", err)
	}
	url := fmt.Sprintf("%s/key-mappings/%s", s.BaseURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Siglet Mapping request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	defer func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if err != nil {
		return nil, fmt.Errorf("failed to get Siglet Mapping: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get Siglet Mapping: received status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var keyMapping KeyMapping
	if err := json.Unmarshal(body, &keyMapping); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key mapping: %w", err)
	}

	return &keyMapping, nil
}

func (s httpManagementApiClient) DeleteKeyMapping(ctx context.Context, participantContextID string) error {

	accessToken, err := s.tokenProvider.GetToken(ctx, ScopeApiWrite, participantContextID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}
	url := fmt.Sprintf("%s/key-mappings/%s", s.BaseURL, participantContextID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create Siglet Mapping request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	defer func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if err != nil {
		return fmt.Errorf("failed to get Siglet Mapping: %w", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error deleting Siglet Mapping: received status code %d", resp.StatusCode)
	}
	return nil
}
