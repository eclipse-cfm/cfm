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

package tokenexchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/eclipse-cfm/cfm/common/system"
)

type tokenExchangeResponse struct {
	AccessToken     string `json:"access_token"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in"`
	Scope           string `json:"scope"`
	IssuedTokenType string `json:"issued_token_type"`
}

// TokenExchangeProvider reads a token from a file share and exchanges it for a resource-bound token. The initial token is a
// projected token bound to the workload ID (in Kubernetes: ServiceAccount), and the second token is a token bound to the
// participant context
type TokenExchangeProvider struct {
	filePath              string
	tokenExchangeUrl      string
	tokenExchangeAudience string
	httpClient            *http.Client
	monitor               system.LogMonitor
}

// ProviderOption is a functional option for configuring TokenExchangeProvider
type ProviderOption func(*TokenExchangeProvider)

// WithTokenExchangeUrl sets the URL of the token exchange server (jwtlet)
func WithTokenExchangeUrl(url string) ProviderOption {
	return func(t *TokenExchangeProvider) {
		t.tokenExchangeUrl = url
	}
}

// WithTokenExchangeAudience sets the token exchange audience (e.g. "edcv"). all downstream services must validate against this audience
func WithTokenExchangeAudience(audience string) ProviderOption {
	return func(t *TokenExchangeProvider) {
		t.tokenExchangeAudience = audience
	}
}

// WithHttpClient sets the HTTP client
func WithHttpClient(client *http.Client) ProviderOption {
	return func(t *TokenExchangeProvider) {
		t.httpClient = client
	}
}

// WithMonitor sets the log monitor
func WithMonitor(monitor system.LogMonitor) ProviderOption {
	return func(t *TokenExchangeProvider) {
		t.monitor = monitor
	}
}

// NewTokenExchangeProvider creates a new TokenExchangeProvider with the given token file path and options
// tokenFilePath: the absolute path to the file that contains the token
func NewTokenExchangeProvider(tokenFilePath string, opts ...ProviderOption) TokenExchangeProvider {
	provider := TokenExchangeProvider{
		filePath: tokenFilePath,
		monitor:  system.NoopMonitor{},
	}
	for _, opt := range opts {
		opt(&provider)
	}
	return provider
}

func (t TokenExchangeProvider) GetToken(ctx context.Context, scope string, participantContextID string) (string, error) {
	t.monitor.Debugw("reading subject token from file", "path", t.filePath)
	content, err := os.ReadFile(t.filePath)
	if err != nil {
		return "", err
	}

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", string(content))
	formData.Set("resource", participantContextID)
	formData.Set("scope", scope)
	formData.Set("audience", t.tokenExchangeAudience)

	t.monitor.Debugw("sending token exchange request", "url", t.tokenExchangeUrl, "scope", scope, "resource", participantContextID, "audience", t.tokenExchangeAudience)
	req, err := http.NewRequestWithContext(ctx, "POST", t.tokenExchangeUrl+"/token", strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("error creating token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending token exchange request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.monitor.Debugw("token exchange failed", "status", resp.StatusCode, "body", string(bodyBytes))
		return "", fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}
	var tokenResponse tokenExchangeResponse
	err = json.Unmarshal(bodyBytes, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("error decoding token exchange response: %w", err)
	}

	t.monitor.Debugw("token exchange successful", "scope", scope, "resource", participantContextID, "token_type", tokenResponse.TokenType, "expires_in", tokenResponse.ExpiresIn)
	return tokenResponse.AccessToken, nil
}
