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

package siglet

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func sampleMapping() TransferTypeMapping {
	return TransferTypeMapping{
		ParticipantContextID: "participant-1",
		Mappings: map[string]TransferType{
			"HttpData-PULL": {
				TransferType:     "HttpData-PULL",
				EndpointType:     "HTTP",
				TokenSource:      "provider",
				Endpoint:         "https://data.provider.example.com/assets",
				TxRenewalSupport: true,
			},
		},
	}
}

func TestHttpApiClient_CreateTransferTypeMapping(t *testing.T) {
	var received TransferTypeMapping
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/transfer-type-mappings" {
			assert.Equal(t, "Bearer test token", r.Header.Get("Authorization"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.CreateTransferTypeMapping(t.Context(), sampleMapping())
	require.NoError(t, err)
	assert.Equal(t, "participant-1", received.ParticipantContextID)
	assert.Equal(t, "HTTP", received.Mappings["HttpData-PULL"].EndpointType)
	assert.True(t, received.Mappings["HttpData-PULL"].TxRenewalSupport)
}

func TestHttpApiClient_CreateTransferTypeMapping_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("auth boom"))
	client := NewTransferTypeMappingClient(&http.Client{}, tp, "http://foo.bar")

	err := client.CreateTransferTypeMapping(t.Context(), sampleMapping())
	require.ErrorContains(t, err, "auth boom")
}

func TestHttpApiClient_CreateTransferTypeMapping_ApiReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.CreateTransferTypeMapping(t.Context(), sampleMapping())
	require.ErrorContains(t, err, "received status code 409")
}

func TestHttpApiClient_GetTransferTypeMapping(t *testing.T) {
	expected := sampleMapping()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/transfer-type-mappings/participant-1" {
			assert.Equal(t, "Bearer test token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expected)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	result, err := client.GetTransferTypeMapping(t.Context(), "participant-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expected, *result)
}

func TestHttpApiClient_GetTransferTypeMapping_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	result, err := client.GetTransferTypeMapping(t.Context(), "participant-1")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestHttpApiClient_GetTransferTypeMapping_InvalidJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	result, err := client.GetTransferTypeMapping(t.Context(), "participant-1")
	require.ErrorContains(t, err, "failed to unmarshal transfer type mapping")
	assert.Nil(t, result)
}

func TestHttpApiClient_ReplaceTransferTypeMapping(t *testing.T) {
	var received TransferTypeMapping
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/transfer-type-mappings/participant-1" {
			assert.Equal(t, "Bearer test token", r.Header.Get("Authorization"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.ReplaceTransferTypeMapping(t.Context(), sampleMapping())
	require.NoError(t, err)
	assert.Equal(t, "provider", received.Mappings["HttpData-PULL"].TokenSource)
}

func TestHttpApiClient_ReplaceTransferTypeMapping_ApiReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.ReplaceTransferTypeMapping(t.Context(), sampleMapping())
	require.ErrorContains(t, err, "received status code 500")
}

func TestHttpApiClient_DeleteTransferTypeMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/transfer-type-mappings/participant-1" {
			assert.Equal(t, "Bearer test token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.DeleteTransferTypeMapping(t.Context(), "participant-1")
	require.NoError(t, err)
}

func TestHttpApiClient_DeleteTransferTypeMapping_NotFoundIsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.DeleteTransferTypeMapping(t.Context(), "participant-1")
	require.NoError(t, err)
}

func TestHttpApiClient_DeleteTransferTypeMapping_ApiReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewTransferTypeMappingClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.DeleteTransferTypeMapping(t.Context(), "participant-1")
	require.ErrorContains(t, err, "received status code 400")
}
