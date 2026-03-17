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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eclipse-cfm/cfm/agent/edcv"
	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/stretchr/testify/require"
)

func TestIdentityAPIClient_CreateParticipantContext(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1alpha/participants" && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var data map[string]any
			err = json.Unmarshal(body, &data)
			require.NoError(t, err)

			require.Equal(t, "test", data["participantContextId"])
			require.Equal(t, "did:web:test", data["did"])
			require.True(t, data["active"].(bool))

			serviceEndpoints := data["serviceEndpoints"].([]any)
			require.Len(t, serviceEndpoints, 2)

			credentialService := serviceEndpoints[0].(map[string]any)
			require.Equal(t, "CredentialService", credentialService["type"])
			require.Equal(t, "https://example.com/credentials", credentialService["serviceEndpoint"])

			protocolService := serviceEndpoints[1].(map[string]any)
			require.Equal(t, "ProtocolEndpoint", protocolService["type"])
			require.Equal(t, "https://example.com/dsp", protocolService["serviceEndpoint"])

			additionalProps := data["additionalProperties"].(map[string]any)
			vaultConfig := additionalProps["edc.vault.hashicorp.config"].(map[string]any)

			credentials := vaultConfig["credentials"].(map[string]any)
			require.Equal(t, "client-id", credentials["clientId"])
			require.Equal(t, "secret", credentials["clientSecret"])
			require.Equal(t, "https://example.com/idp/token", credentials["tokenUrl"])

			config := vaultConfig["config"].(map[string]any)
			require.Equal(t, "https://example.com/vault", config["vaultUrl"])
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"participantContextId":"test"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	manifest := NewParticipantManifest("test", "did:web:test", "https://example.com/credentials", "https://example.com/dsp",
		func(manifest *ParticipantManifest) {
			manifest.VaultConfig = edcv.VaultConfig{
				VaultURL: "https://example.com/vault",
			}
			manifest.VaultCredentials.ClientSecret = "secret"
			manifest.VaultCredentials.ClientID = "client-id"
			manifest.VaultCredentials.TokenURL = "https://example.com/idp/token"
		})
	_, err := client.CreateParticipantContext(manifest)
	require.NoError(t, err)

}

func TestIdentityAPIClient_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("", fmt.Errorf("test error"))
	client := HttpIdentityAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	manifest := NewParticipantManifest("test", "did:web:test", "https://example.com/credentials", "https://example.com/dsp",
		func(manifest *ParticipantManifest) {
			manifest.VaultConfig = edcv.VaultConfig{
				VaultURL: "https://example.com/vault",
			}
			manifest.VaultCredentials.ClientSecret = "secret"
			manifest.VaultCredentials.ClientID = "client-id"
			manifest.VaultCredentials.TokenURL = "https://example.com/idp/token"
		})
	_, err := client.CreateParticipantContext(manifest)
	require.ErrorContains(t, err, "failed to get API access token: test error")
}

func TestIdentityAPIClient_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	manifest := NewParticipantManifest("test", "did:web:test", "https://example.com/credentials", "https://example.com/dsp",
		func(manifest *ParticipantManifest) {
			manifest.VaultConfig = edcv.VaultConfig{
				VaultURL: "https://example.com/vault",
			}
			manifest.VaultCredentials.ClientSecret = "secret"
			manifest.VaultCredentials.ClientID = "client-id"
			manifest.VaultCredentials.TokenURL = "https://example.com/idp/token"
		})
	_, err := client.CreateParticipantContext(manifest)
	require.ErrorContains(t, err, "foobar")
}

func TestHttpIdentityAPIClient_DeleteParticipantContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		participantContextID := "test-id"
		if r.URL.Path == "/v1alpha/participants/"+participantContextID && r.Method == http.MethodDelete {
			require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.DeleteParticipantContext("test-id")
	require.NoError(t, err)
}

func TestHttpIdentityAPIClient_DeleteParticipantContext_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("participant not found"))
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.DeleteParticipantContext("test")
	require.ErrorContains(t, err, "participant not found")
}
