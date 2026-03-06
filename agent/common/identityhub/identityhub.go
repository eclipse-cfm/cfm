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

package identityhub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/metaform/connector-fabric-manager/common/token"
)

const (
	CreateParticipantURL = "/v1alpha/participants"
)

const (
	CredentialRequestStateCreated  = "CREATED"
	CredentialRequestStateIssued   = "ISSUED"
	CredentialRequestStateRejected = "REJECTED"
	CredentialRequestStateError    = "ERROR"
)

type IdentityAPIClient interface {
	CreateParticipantContext(manifest ParticipantManifest) (*CreateParticipantContextResponse, error)
	RequestCredentials(participantContextID string, credentialRequest CredentialRequest) (string, error)
	GetCredentialRequestState(participantContextID string, credentialRequestID string) (string, error)
	QueryCredentialByType(participantContextID string, credentialType string) ([]VerifiableCredentialResource, error)
	DeleteParticipantContext(participantContextID string) error
}

type HttpIdentityAPIClient struct {
	BaseURL       string
	TokenProvider token.TokenProvider
	HttpClient    *http.Client
}

func (a HttpIdentityAPIClient) DeleteParticipantContext(participantContextID string) error {
	accessToken, err := a.TokenProvider.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/v1alpha/participants/%s", a.BaseURL, participantContextID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.HttpClient.Do(req)
	defer a.closeResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to delete participant context on IdentityHub: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("participant context %s not found in Identity", participantContextID)
	case http.StatusOK:
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete participant context on IdentityHub: received status code %d, body: %s", resp.StatusCode, string(body))
	}
}

func (a HttpIdentityAPIClient) QueryCredentialByType(participantContextID string, credentialType string) ([]VerifiableCredentialResource, error) {

	accessToken, err := a.TokenProvider.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/v1alpha/participants/%s/credentials?type=%s", a.BaseURL, participantContextID, credentialType)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.HttpClient.Do(req)
	defer a.closeResponse(resp)

	if err != nil {
		return nil, fmt.Errorf("failed to get credential request state for %s: %w", participantContextID, err)
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get credential request state: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	var credentials []VerifiableCredentialResource
	if err := json.Unmarshal(body, &credentials); err != nil {
		return nil, fmt.Errorf("failed to unmarshal verifiable credentials array: %w", err)
	}

	return credentials, nil
}

func (a HttpIdentityAPIClient) RequestCredentials(participantContextID string, credentialRequest CredentialRequest) (string, error) {
	accessToken, err := a.TokenProvider.GetToken() // this should be the participant context's access token!
	if err != nil {
		return "", fmt.Errorf("failed to get API access token: %w", err)
	}

	payload, err := json.Marshal(credentialRequest)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/v1alpha/participants/%s/credentials/request", a.BaseURL, participantContextID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.HttpClient.Do(req)
	defer a.closeResponse(resp)

	if err != nil {
		return "", fmt.Errorf("failed to create credentials request for %s: %w", participantContextID, err)
	}

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to request credentials for participant context on IdentityHub: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	location := resp.Header.Get("Location")

	return location, nil
}

func (a HttpIdentityAPIClient) GetCredentialRequestState(participantContextID string, credentialRequestID string) (string, error) {
	accessToken, err := a.TokenProvider.GetToken()
	if err != nil {
		return "", fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/v1alpha/participants/%s/credentials/request/%s", a.BaseURL, participantContextID, credentialRequestID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.HttpClient.Do(req)
	defer a.closeResponse(resp)

	if err != nil {
		return "", fmt.Errorf("failed to get credential request state for %s: %w", participantContextID, err)
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get credential request state: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	var stateResponse map[string]interface{}
	if err := json.Unmarshal(body, &stateResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal credential request state response: %w", err)
	}

	stateStr, ok := stateResponse["status"].(string)
	if !ok {
		return "", fmt.Errorf("invalid status format in response")
	}

	return stateStr, nil
}

func (a HttpIdentityAPIClient) CreateParticipantContext(manifest ParticipantManifest) (*CreateParticipantContextResponse, error) {
	accessToken, err := a.TokenProvider.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get API access token: %w", err)
	}

	data := map[string]any{
		"roles": []string{"participant"},
		"serviceEndpoints": []map[string]any{
			{
				"type":            "CredentialService",
				"id":              manifest.CredentialServiceID,
				"serviceEndpoint": manifest.CredentialServiceURL,
			},
			{
				"type":            "ProtocolEndpoint",
				"id":              manifest.ProtocolServiceID,
				"serviceEndpoint": manifest.ProtocolServiceURL,
			},
		},
		"active":               manifest.IsActive,
		"participantContextId": manifest.ParticipantContextID,
		"did":                  manifest.DID,
		"key": map[string]any{
			"keyId":           manifest.KeyGeneratorParameters.KeyID,
			"privateKeyAlias": manifest.KeyGeneratorParameters.PrivateKeyAlias,
			"keyGeneratorParams": map[string]string{
				"algorithm": manifest.KeyGeneratorParameters.KeyAlgorithm,
				"curve":     manifest.KeyGeneratorParameters.Curve,
			},
		},
		"additionalProperties": map[string]any{
			"edc.vault.hashicorp.config": map[string]any{
				"credentials": map[string]string{
					"clientId":     manifest.VaultCredentials.ClientID,
					"clientSecret": manifest.VaultCredentials.ClientSecret,
					"tokenUrl":     manifest.VaultCredentials.TokenURL,
				},
				"config": map[string]string{
					"secretPath": manifest.VaultConfig.SecretPath,
					"folderPath": manifest.VaultConfig.FolderPath,
					"vaultUrl":   manifest.VaultConfig.VaultURL,
				},
			},
		},
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	url := a.BaseURL + CreateParticipantURL
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create participant context on IdentityHub: %w", err)
	}
	defer a.closeResponse(resp)

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to create participant context on IdentityHub: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	createResponse := &CreateParticipantContextResponse{}

	if err := json.Unmarshal(body, createResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal participant context creation response: %w", err)
	}

	return createResponse, nil
}

func (a HttpIdentityAPIClient) closeResponse(resp *http.Response) {
	func() {
		// drain and close response body to avoid connection/resource leak
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
}
