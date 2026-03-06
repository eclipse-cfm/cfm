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

package oauth2

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/eclipse-cfm/cfm/common/runtime"
)

const (
	ClientCredentials = "client_credentials"
	contentTypeHeader = "Content-Type"
)

// TokenProvider can generate OAuth2/JWT tokens based on either a client-credentials grant or a password grant
type TokenProvider struct {
	tokenParams Oauth2Params
	httpClient  *http.Client
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

type Oauth2Params struct {
	ClientID     string
	ClientSecret string
	UserName     string
	Password     string
	TokenURL     string
	GrantType    string
	Scope        string
}

func NewTokenProvider(params Oauth2Params, client *http.Client) *TokenProvider {
	return &TokenProvider{
		tokenParams: params,
		httpClient:  client,
	}
}

// GetToken gets an OAuth2 token using the grant type specified in the Oauth2Params
func (t *TokenProvider) GetToken() (string, error) {
	switch t.tokenParams.GrantType {
	case "client_credentials":
		return t.getClientCredentialsToken()
	case "password":
		return t.getPasswordCredentialsToken()
	default:
		return "", fmt.Errorf("grant Type '%s' not supported", t.tokenParams.GrantType)
	}
}

func (t *TokenProvider) getClientCredentialsToken() (string, error) {
	tokenURL := t.tokenParams.TokenURL
	clientID := t.tokenParams.ClientID
	clientSecret := t.tokenParams.ClientSecret

	err := runtime.CheckRequiredParams("tokenURL", tokenURL, "clientID", clientID, "clientSecret", clientSecret)
	if err != nil {
		return "", err
	}

	formData := fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=%s", clientID, clientSecret, ClientCredentials)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("error creating token request: %w", err)
	}

	return t.executeTokenRequest(req)
}

func (t *TokenProvider) getPasswordCredentialsToken() (string, error) {
	username := t.tokenParams.UserName
	password := t.tokenParams.Password
	clientId := t.tokenParams.ClientID
	tokenURL := t.tokenParams.TokenURL

	err := runtime.CheckRequiredParams("username", username, "password", password, "clientId", clientId, "tokenURL", tokenURL)
	if err != nil {
		return "", err
	}

	formData := fmt.Sprintf("username=%s&password=%s&client_id=%s&grant_type=password",
		username, password, clientId)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("error creating token request: %w", err)
	}

	return t.executeTokenRequest(req)
}

func (t *TokenProvider) executeTokenRequest(req *http.Request) (string, error) {
	req.Header.Set(contentTypeHeader, "application/x-www-form-urlencoded")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("error decoding token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}
