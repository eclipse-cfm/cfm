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

package issuerservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/eclipse-cfm/cfm/common/token"
)

type ApiClient interface {
	CreateHolder(ctx context.Context, did string, holderID string, name string) error
	DeleteHolder(ctx context.Context, holderID string) error
	RevokeCredential(ctx context.Context, participantContextID string, credentialID string) error
}

type HttpApiClient struct {
	BaseURL       string
	TokenProvider token.TokenProvider
	IssuerID      string
	HttpClient    *http.Client
}

func (i HttpApiClient) DeleteHolder(ctx context.Context, holderID string) error {
	accessToken, err := i.TokenProvider.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/v1alpha/participants/%s/holders/%s", i.BaseURL, i.IssuerID, holderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := i.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete Holder on IssuerService: %w", err)
	}
	defer func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete Holder on IssuerService: received status code %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (i HttpApiClient) CreateHolder(ctx context.Context, did string, holderID string, name string) error {
	accessToken, err := i.TokenProvider.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	data := map[string]any{
		"did":      did,
		"holderId": holderID,
		"name":     name,
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling payload: %w", err)
	}

	url := fmt.Sprintf("%s/v1alpha/participants/%s/holders", i.BaseURL, i.IssuerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := i.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create Holder on IssuerService: %w", err)
	}
	defer func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create Holder on ApiClient: received status code %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (i HttpApiClient) RevokeCredential(ctx context.Context, participantContextID string, credentialID string) error {
	accessToken, err := i.TokenProvider.GetToken(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v1alpha/participants/%s/credentials/%s/revoke", i.BaseURL, participantContextID, credentialID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := i.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to revoke credential with ID '%s' for participant context '%s': %w", credentialID, participantContextID, err)
	}
	defer func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to revoke credential with ID '%s' for participant context '%s': received status code %d with message [%s]", credentialID, participantContextID, resp.StatusCode, string(body))
	}
	return nil
}
