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

	"github.com/eclipse-cfm/cfm/agent/orchestration/edcv"
	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIdentityAPIClient_CreateParticipantContext(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/participants" && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var data map[string]any
			err = json.Unmarshal(body, &data)
			require.NoError(t, err)

			require.Equal(t, "test", data["participantContextId"])
			require.Equal(t, "did:web:test", data["did"])
			require.True(t, data["active"].(bool))
			require.False(t, data["provisionStsAccount"].(bool))
			require.False(t, data["provisionApiKey"].(bool))

			serviceEndpoints := data["serviceEndpoints"].([]any)
			require.Len(t, serviceEndpoints, 3)

			credentialService := serviceEndpoints[0].(map[string]any)
			require.Equal(t, "CredentialService", credentialService["type"])
			require.Equal(t, "https://example.com/credentials", credentialService["serviceEndpoint"])

			protocolService := serviceEndpoints[1].(map[string]any)
			require.Equal(t, "ProtocolEndpoint", protocolService["type"])
			require.Equal(t, "https://example.com/dsp", protocolService["serviceEndpoint"])

			dataService := serviceEndpoints[2].(map[string]any)
			require.Equal(t, "DataService", dataService["type"])
			require.Equal(t, "https://example.com/data", dataService["serviceEndpoint"])

			additionalProps := data["additionalProperties"].(map[string]any)
			vaultConfig := additionalProps["edc.vault.hashicorp.config"].(map[string]any)

			require.NotContains(t, vaultConfig, "credentials")

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
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	manifest := NewParticipantManifest("test", "did:web:test", "https://example.com/credentials", "https://example.com/dsp", "https://example.com/data",
		func(manifest *ParticipantManifest) {
			manifest.VaultConfig = edcv.VaultConfig{
				VaultURL: "https://example.com/vault",
			}
		})
	err := client.CreateParticipantContext(t.Context(), manifest)
	require.NoError(t, err)

}

func TestIdentityAPIClient_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test error"))
	client := HttpIdentityAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	manifest := NewParticipantManifest("test", "did:web:test", "https://example.com/credentials", "https://example.com/dsp", "https://example.com/data",
		func(manifest *ParticipantManifest) {
			manifest.VaultConfig = edcv.VaultConfig{
				VaultURL: "https://example.com/vault",
			}
		})
	err := client.CreateParticipantContext(t.Context(), manifest)
	require.ErrorContains(t, err, "failed to get API access token: test error")
}

func TestIdentityAPIClient_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	manifest := NewParticipantManifest("test", "did:web:test", "https://example.com/credentials", "https://example.com/dsp", "https://example.com/data",
		func(manifest *ParticipantManifest) {
			manifest.VaultConfig = edcv.VaultConfig{
				VaultURL: "https://example.com/vault",
			}
		})
	err := client.CreateParticipantContext(t.Context(), manifest)
	require.ErrorContains(t, err, "foobar")
}

func TestHttpIdentityAPIClient_DeleteParticipantContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		participantContextID := "test-id"
		if r.URL.Path == "/v1/participants/"+participantContextID && r.Method == http.MethodDelete {
			require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.DeleteParticipantContext(t.Context(), "test-id")
	require.NoError(t, err)
}

func TestHttpIdentityAPIClient_DeleteParticipantContext_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("participant not found"))
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.DeleteParticipantContext(t.Context(), "test")
	require.ErrorContains(t, err, "participant context test not found")
}

func TestHolderPIDFromLocation(t *testing.T) {
	holderPID, err := HolderPIDFromLocation("v1/participants/participant-1/credentials/request/holder-123")
	require.NoError(t, err)
	require.Equal(t, "holder-123", holderPID)

	holderPID, err = HolderPIDFromLocation("http://identityhub/api/identity/v1/participants/participant-1/credentials/request/holder-123")
	require.NoError(t, err)
	require.Equal(t, "holder-123", holderPID)
}

func TestHolderPIDFromLocation_Invalid(t *testing.T) {
	for _, location := range []string{"", "/", "v1/participants/participant-1/credentials/request", "http://[::1"} {
		_, err := HolderPIDFromLocation(location)
		require.Error(t, err, location)
	}
}

func TestHttpIdentityAPIClient_TransportError(t *testing.T) {
	// a closed server makes every request fail at the transport level, so the client gets a nil response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpIdentityAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	tests := map[string]func() error{
		"CreateParticipantContext": func() error {
			return client.CreateParticipantContext(t.Context(), NewParticipantManifest("test", "did:web:test", "https://example.com/credentials", "https://example.com/dsp", "https://example.com/data"))
		},
		"DeleteParticipantContext": func() error {
			return client.DeleteParticipantContext(t.Context(), "test")
		},
		"QueryCredentialByType": func() error {
			_, err := client.QueryCredentialByType(t.Context(), "test", "MembershipCredential")
			return err
		},
		"RequestCredentials": func() error {
			_, err := client.RequestCredentials(t.Context(), "test", CredentialRequest{})
			return err
		},
		"GetCredentialRequestState": func() error {
			_, err := client.GetCredentialRequestState(t.Context(), "test", "request-1")
			return err
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			require.NotPanics(t, func() {
				require.Error(t, call())
			})
		})
	}
}
