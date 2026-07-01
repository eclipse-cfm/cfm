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

func newTokenProvider(t *testing.T) *mocks.MockTokenProvider {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("test token", nil)
	return tp
}

func TestHttpApiClient_CreateKeyMapping(t *testing.T) {
	var received KeyMapping
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/key-mappings" {
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

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.CreateKeyMapping(t.Context(), KeyMapping{
		ParticipantContextID: "participant-1",
		KeyName:              "key-1",
		KeyID:                "kid-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "participant-1", received.ParticipantContextID)
	assert.Equal(t, "key-1", received.KeyName)
	assert.Equal(t, "kid-1", received.KeyID)
}

func TestHttpApiClient_CreateKeyMapping_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("auth boom"))
	client := NewSigletAPIClient(&http.Client{}, tp, "http://foo.bar")

	err := client.CreateKeyMapping(t.Context(), KeyMapping{ParticipantContextID: "participant-1"})
	require.ErrorContains(t, err, "auth boom")
}

func TestHttpApiClient_CreateKeyMapping_ApiReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.CreateKeyMapping(t.Context(), KeyMapping{ParticipantContextID: "participant-1"})
	require.ErrorContains(t, err, "received status code 400")
}

func TestHttpApiClient_UpdateKeyMapping(t *testing.T) {
	var received KeyMapping
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/key-mappings/participant-1" {
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

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.UpdateKeyMapping(t.Context(), KeyMapping{
		ParticipantContextID: "participant-1",
		KeyName:              "key-2",
	})
	require.NoError(t, err)
	assert.Equal(t, "key-2", received.KeyName)
}

func TestHttpApiClient_UpdateKeyMapping_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("auth boom"))
	client := NewSigletAPIClient(&http.Client{}, tp, "http://foo.bar")

	err := client.UpdateKeyMapping(t.Context(), KeyMapping{ParticipantContextID: "participant-1"})
	require.ErrorContains(t, err, "auth boom")
}

func TestHttpApiClient_UpdateKeyMapping_ApiReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.UpdateKeyMapping(t.Context(), KeyMapping{ParticipantContextID: "participant-1"})
	require.ErrorContains(t, err, "received status code 500")
}

func TestHttpApiClient_GetKeyMapping(t *testing.T) {
	expected := KeyMapping{
		ParticipantContextID: "participant-1",
		KeyName:              "key-1",
		KeyID:                "kid-1",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/key-mappings/participant-1" {
			assert.Equal(t, "Bearer test token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(expected)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	result, err := client.GetKeyMapping(t.Context(), "participant-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expected, *result)
}

func TestHttpApiClient_GetKeyMapping_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	result, err := client.GetKeyMapping(t.Context(), "participant-1")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestHttpApiClient_GetKeyMapping_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("auth boom"))
	client := NewSigletAPIClient(&http.Client{}, tp, "http://foo.bar")

	result, err := client.GetKeyMapping(t.Context(), "participant-1")
	require.ErrorContains(t, err, "auth boom")
	assert.Nil(t, result)
}

func TestHttpApiClient_GetKeyMapping_ApiReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	result, err := client.GetKeyMapping(t.Context(), "participant-1")
	require.ErrorContains(t, err, "received status code 500")
	assert.Nil(t, result)
}

func TestHttpApiClient_GetKeyMapping_InvalidJson(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	result, err := client.GetKeyMapping(t.Context(), "participant-1")
	require.ErrorContains(t, err, "failed to unmarshal key mapping")
	assert.Nil(t, result)
}

func TestHttpApiClient_DeleteKeyMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/key-mappings/participant-1" {
			assert.Equal(t, "Bearer test token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.DeleteKeyMapping(t.Context(), "participant-1")
	require.NoError(t, err)
}

func TestHttpApiClient_DeleteKeyMapping_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("auth boom"))
	client := NewSigletAPIClient(&http.Client{}, tp, "http://foo.bar")

	err := client.DeleteKeyMapping(t.Context(), "participant-1")
	require.ErrorContains(t, err, "auth boom")
}

func TestHttpApiClient_DeleteKeyMapping_ApiReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := NewSigletAPIClient(&http.Client{}, newTokenProvider(t), server.URL)
	err := client.DeleteKeyMapping(t.Context(), "participant-1")
	require.ErrorContains(t, err, "received status code 400")
}
