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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
)

type tokenExchangeRequest struct {
	GrantType    string `json:"grant_type"`
	SubjectToken string `json:"subject_token"`
	Resource     string `json:"resource"`
	Scope        string `json:"scope"`
	Audience     string `json:"audience"`
}

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
}

func NewTokenExchangeProvider(filename string) TokenExchangeProvider {
	return TokenExchangeProvider{
		filePath: filename,
	}
}

func (t TokenExchangeProvider) GetToken(ctx context.Context) (string, error) {
	content, err := os.ReadFile(t.filePath)
	if err != nil {
		return "", err
	}

	body := tokenExchangeRequest{
		GrantType:    "urn:ietf:params:oauth:grant-type:token-exchange",
		SubjectToken: string(content),
		Resource:     "did:web:foobar",
		Scope:        "read write",
		Audience:     t.tokenExchangeAudience,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", t.tokenExchangeUrl, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.httpClient.Do(req)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResponse tokenExchangeResponse
	err = json.NewDecoder(resp.Body).Decode(&tokenResponse)
	if err != nil {
		return "", err
	}

	return tokenResponse.AccessToken, nil
}
