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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/agent/common"
	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHttpApiClient_CreateHolder(t *testing.T) {
	template := "/v1alpha/participants/.*/holders"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodPost && matched {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var data map[string]any
			err = json.Unmarshal(body, &data)

			require.Equal(t, "did:web:test-participant", data["did"])
			require.Equal(t, "did:web:test-participant", data["holderId"])
			require.Equal(t, "test holder", data["name"])

			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.CreateHolder(t.Context(), "did:web:test-participant", "did:web:test-participant", "test holder", nil)
	require.NoError(t, err)
}

func TestHttpApiClient_CreateHolder_AuthError(t *testing.T) {

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("", fmt.Errorf("test error"))
	client := HttpApiClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.CreateHolder(t.Context(), "did:web:test-participant", "did:web:test-participant", "test holder", nil)
	require.ErrorContains(t, err, "test error")
}

func TestHttpApiClient_CreateHolder_ApiReturnsError(t *testing.T) {
	template := "/v1alpha/participants/.*/holders"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodPost && matched {

			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.CreateHolder(t.Context(), "did:web:test-participant", "did:web:test-participant", "test holder", nil)
	require.ErrorContains(t, err, "failed to create Holder")
}

func TestHttpApiClient_RevokeCredential(t *testing.T) {
	template := "/v1alpha/participants/.*/credentials/.*/revoke"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if matched && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.RevokeCredential(t.Context(), "did:web:test-participant", "test-credential-id")
	require.NoError(t, err)
}

func TestHttpApiClient_RevokeCredential_ClientError(t *testing.T) {
	template := "/v1alpha/participants/.*/credentials/.*/revoke"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if matched && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.RevokeCredential(t.Context(), "did:web:test-participant", "test-credential-id")
	require.ErrorContains(t, err, "failed to revoke credential")
}

func TestHttpApiClient_RevokeCredential_NotFound(t *testing.T) {
	template := "/v1alpha/participants/.*/credentials/.*/revoke"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if matched && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.RevokeCredential(t.Context(), "did:web:test-participant", "test-credential-id")
	require.ErrorContains(t, err, "failed to revoke credential")
}

func TestHttpApiClient_DeleteHolder(t *testing.T) {
	template := "/v1alpha/participants/.*/holders/.*"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodDelete && matched {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.DeleteHolder(t.Context(), "did:web:test-participant")
	require.NoError(t, err)
}

func TestHttpApiClient_DeleteHolder_NotFound(t *testing.T) {
	template := "/v1alpha/participants/.*/holders/.*"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodDelete && matched {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.DeleteHolder(t.Context(), "did:web:test-participant")
	require.ErrorContains(t, err, "received status code 404")
}

func TestHttpApiClient_DeleteHolder_AuthError(t *testing.T) {
	template := "/v1alpha/participants/.*/holders/.*"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodDelete && matched {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	err := client.DeleteHolder(t.Context(), "did:web:test-participant")
	require.ErrorContains(t, err, "received status code 401")
}

func TestHttpApiClient_QueryCredentialsByType(t *testing.T) {
	template := "/v1alpha/participants/.*/credentials/query"
	credentials := []IssuerCredentialResourceDto{
		{
			ID:                   "cred-1",
			ParticipantContextID: "did:web:test-participant",
			CredentialFormat:     common.CredentialFormat_VCDM10_JWT,
			VerifiableCredential: common.VerifiableCredential{
				ID:           "vc-1",
				Types:        []string{"VerifiableCredential", "MembershipCredential"},
				Issuer:       common.Issuer{ID: "did:web:issuer"},
				IssuanceDate: time.Now(),
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodPost && matched {
			var body common.QuerySpec
			err := json.NewDecoder(r.Body).Decode(&body)
			require.NoError(t, err)
			require.Len(t, body.FilterExpression, 2)
			assert.Equal(t, "verifiableCredential.credential.type", body.FilterExpression[0].OperandLeft)
			assert.Equal(t, "contains", body.FilterExpression[0].Operator)
			assert.Equal(t, "MembershipCredential", body.FilterExpression[0].OperandRight)
			assert.Equal(t, "holderId", body.FilterExpression[1].OperandLeft)
			assert.Equal(t, "=", body.FilterExpression[1].Operator)
			assert.Equal(t, "did:web:test-participant", body.FilterExpression[1].OperandRight)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(credentials)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	result, err := client.QueryCredentialsByType(t.Context(), "did:web:test-participant", "MembershipCredential")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "cred-1", result[0].ID)
	assert.Equal(t, "did:web:test-participant", result[0].ParticipantContextID)
}

func TestHttpApiClient_QueryCredentialsByType_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("", fmt.Errorf("auth error"))
	client := HttpApiClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	result, err := client.QueryCredentialsByType(t.Context(), "did:web:test-participant", "MembershipCredential")
	require.ErrorContains(t, err, "auth error")
	require.Nil(t, result)
}

func TestHttpApiClient_QueryCredentialsByType_ApiReturnsError(t *testing.T) {
	template := "/v1alpha/participants/.*/credentials/query"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodPost && matched {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	result, err := client.QueryCredentialsByType(t.Context(), "did:web:test-participant", "MembershipCredential")
	require.ErrorContains(t, err, "failed to query credentials on IssuerService")
	require.Nil(t, result)
}

func TestHttpApiClient_QueryCredentialsByType_InvalidJson(t *testing.T) {
	template := "/v1alpha/participants/.*/credentials/query"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		matched, _ := regexp.MatchString(template, r.URL.Path)
		if r.Method == http.MethodPost && matched {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not valid json"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything).Return("test token", nil)
	client := HttpApiClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		IssuerID:      "test-issuer",
		HttpClient:    &http.Client{},
	}

	result, err := client.QueryCredentialsByType(t.Context(), "did:web:test-participant", "MembershipCredential")
	require.ErrorContains(t, err, "error parsing credentials response")
	require.Nil(t, result)
}
