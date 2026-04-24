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

	"github.com/eclipse-cfm/cfm/common/mocks"
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

	err := client.CreateHolder(t.Context(), "did:web:test-participant", "did:web:test-participant", "test holder")
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

	err := client.CreateHolder(t.Context(), "did:web:test-participant", "did:web:test-participant", "test holder")
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

	err := client.CreateHolder(t.Context(), "did:web:test-participant", "did:web:test-participant", "test holder")
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
