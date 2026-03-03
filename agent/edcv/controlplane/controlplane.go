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
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/metaform/connector-fabric-manager/agent/edcv"
	"github.com/metaform/connector-fabric-manager/common/token"
)

const (
	CreateParticipantURL                                       = "/v5alpha/participants"
	applicationJSON                                            = "application/json"
	ParticipantContextStateCreated     ParticipantContextState = "CREATED"
	ParticipantContextStateActivated   ParticipantContextState = "ACTIVATED"
	ParticipantContextStateDeactivated ParticipantContextState = "DEACTIVATED"
	contextConnector                                           = "https://w3id.org/edc/connector/management/v2"
)

type ParticipantContextConfig struct {
	ParticipantContextID string            `json:"participantContextId"`
	Entries              map[string]string `json:"entries"`
	SecretEntries        map[string]string `json:"privateEntries"`
}

func NewParticipantContextConfig(participantContextID string, stsClientID string, stsClientSecretAlias string, participantID string, vConfig edcv.VaultConfig, vCreds edcv.VaultCredentials, stsTokenURL string) ParticipantContextConfig {
	vaultConfig := map[string]any{
		"credentials": vCreds,
		"config":      vConfig,
	}
	return ParticipantContextConfig{
		ParticipantContextID: participantContextID,
		Entries: map[string]string{
			"edc.iam.sts.oauth.token.url":           stsTokenURL,
			"edc.iam.issuer.id":                     participantID,
			"edc.iam.sts.oauth.client.id":           stsClientID,
			"edc.iam.sts.oauth.client.secret.alias": stsClientSecretAlias,
			"edc.participant.id":                    participantID,
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
	CreateParticipantContext(manifest ParticipantContext) error
	CreateConfig(participantContextID string, config ParticipantContextConfig) error
}

type HttpManagementAPIClient struct {
	BaseURL       string
	TokenProvider token.TokenProvider
	HttpClient    *http.Client
}

func (h HttpManagementAPIClient) CreateParticipantContext(manifest ParticipantContext) error {
	accessToken, err := h.TokenProvider.GetToken()
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
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
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
		return fmt.Errorf("failed to create participant context on IdentityHub: received status code %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (h HttpManagementAPIClient) CreateConfig(participantContextID string, config ParticipantContextConfig) error {
	accessToken, err := h.TokenProvider.GetToken()
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
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", applicationJSON)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create participant context config on control plane: %w", err)
	}

	defer h.closeResponse(resp)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create participant context config on control plane: received status code %d, body: %s", resp.StatusCode, string(body))

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
