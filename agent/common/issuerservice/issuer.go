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
 *       Matthias Buchhorn - accept 204 on a credential revocation
 *
 */

package issuerservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/eclipse-cfm/cfm/agent/common"
	"github.com/eclipse-cfm/cfm/common/httpx"
	"github.com/eclipse-cfm/cfm/common/token"
)

// The tokens for the issuer-admin-api are exchanged under the issuer's participant context
// (HttpApiClient.IssuerID), so the token's sub owns every context these calls address and the
// resource scopes below suffice; no issuer-admin-api:admin elevation is needed. The cfm-agents
// ServiceAccount needs a jwtlet mapping on the issuer context granting these scopes, which the
// platform seeds.
const (
	ScopeHoldersWrite     = "issuer-admin-api:holders:write"
	ScopeCredentialsRead  = "issuer-admin-api:credentials:read"
	ScopeCredentialsWrite = "issuer-admin-api:credentials:write"
)

// IssuerCredentialResourceDto represents a DTO for verifiable credentials that the IssuerService has issued to holders.
// note that these DTOs are simplified representations of the actual verifiable credentials and NEVER include the actual
// signed credential
type IssuerCredentialResourceDto struct {
	ID                   string                      `json:"id"`
	ParticipantContextID string                      `json:"participantContextId"`
	CredentialFormat     common.CredentialFormat     `json:"format"`
	VerifiableCredential common.VerifiableCredential `json:"credential"`
}

type ApiClient interface {
	CreateHolder(ctx context.Context, did string, holderID string, name string, properties map[string]any) error
	DeleteHolder(ctx context.Context, holderID string) error
	// RevokeCredential revokes an issued credential. credentialID is the ID of the IssuerService's credential resource
	// (IssuerCredentialResourceDto.ID), not the ID of the verifiable credential itself.
	RevokeCredential(ctx context.Context, credentialID string) error
	// QueryCredentialsByType returns the credentials of the given type that were issued to the holder. The IssuerService
	// records the credential subject's ID (the holder's DID) as the holder ID of an issued credential.
	QueryCredentialsByType(ctx context.Context, holderID string, credentialType string) ([]IssuerCredentialResourceDto, error)
}

type HttpApiClient struct {
	BaseURL       string
	TokenProvider token.TokenProvider
	IssuerID      string
	HttpClient    *http.Client
}

func (i HttpApiClient) QueryCredentialsByType(ctx context.Context, holderID string, credentialType string) ([]IssuerCredentialResourceDto, error) {
	accessToken, err := i.TokenProvider.GetToken(ctx, ScopeCredentialsRead, i.IssuerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/v1/participants/%s/credentials/query", i.BaseURL, url.PathEscape(i.IssuerID))
	body := common.NewQuerySpec(common.WithFilterCriteria(
		common.Criterion{
			OperandLeft:  "verifiableCredential.credential.type",
			Operator:     "contains",
			OperandRight: credentialType,
		},
		common.Criterion{
			OperandLeft:  "holderId",
			Operator:     "=",
			OperandRight: holderID,
		}))

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error marshalling query body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := i.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error querying credentials on IssuerService: %w", err)
	}

	defer httpx.DrainAndClose(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query credentials on IssuerService: received status code %d, body: %s", resp.StatusCode, httpx.ReadErrorBody(resp))
	}

	var credentials []IssuerCredentialResourceDto
	if err := json.NewDecoder(resp.Body).Decode(&credentials); err != nil {
		return nil, fmt.Errorf("error parsing credentials response: %w", err)
	}

	return credentials, nil
}

func (i HttpApiClient) DeleteHolder(ctx context.Context, holderID string) error {
	accessToken, err := i.TokenProvider.GetToken(ctx, ScopeHoldersWrite, i.IssuerID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	url := fmt.Sprintf("%s/v1/participants/%s/holders/%s", i.BaseURL, url.PathEscape(i.IssuerID), url.PathEscape(holderID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := i.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete Holder on IssuerService: %w", err)
	}
	defer httpx.DrainAndClose(resp)

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete Holder on IssuerService: received status code %d, body: %s", resp.StatusCode, httpx.ReadErrorBody(resp))
	}
	return nil
}

func (i HttpApiClient) CreateHolder(ctx context.Context, did string, holderID string, name string, properties map[string]any) error {
	accessToken, err := i.TokenProvider.GetToken(ctx, ScopeHoldersWrite, i.IssuerID)
	if err != nil {
		return fmt.Errorf("failed to get API access token: %w", err)
	}

	data := map[string]any{
		"did":      did,
		"holderId": holderID,
		"name":     name,
	}

	if properties != nil {
		data["properties"] = properties
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling payload: %w", err)
	}

	url := fmt.Sprintf("%s/v1/participants/%s/holders", i.BaseURL, url.PathEscape(i.IssuerID))
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
	defer httpx.DrainAndClose(resp)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create Holder on ApiClient: received status code %d, body: %s", resp.StatusCode, httpx.ReadErrorBody(resp))
	}
	return nil
}

func (i HttpApiClient) RevokeCredential(ctx context.Context, credentialID string) error {
	accessToken, err := i.TokenProvider.GetToken(ctx, ScopeCredentialsWrite, i.IssuerID)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v1/participants/%s/credentials/%s/revoke", i.BaseURL, url.PathEscape(i.IssuerID), url.PathEscape(credentialID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := i.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to revoke credential with ID '%s' in issuer context '%s': %w", credentialID, i.IssuerID, err)
	}
	defer httpx.DrainAndClose(resp)

	// The IssuerService (EDC 1.0) answers a revocation with 204
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to revoke credential with ID '%s' in issuer context '%s': received status code %d with message [%s]", credentialID, i.IssuerID, resp.StatusCode, httpx.ReadErrorBody(resp))
	}
	return nil
}
