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

package tokenexchange

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSubjectToken       = "subject-token-value"
	testScope              = "test-scope:read"
	testParticipantContext = "did:web:test-participant"
	testAudience           = "test-audience"
	testAccessToken        = "exchanged-access-token"
)

func writeTokenFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func tokenResponse() tokenExchangeResponse {
	return tokenExchangeResponse{
		AccessToken:     testAccessToken,
		TokenType:       "Bearer",
		ExpiresIn:       3600,
		Scope:           testScope,
		IssuedTokenType: "urn:ietf:params:oauth:token-type:access_token",
	}
}

func TestGetToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/token", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		assert.Equal(t, "urn:ietf:params:oauth:grant-type:token-exchange", form.Get("grant_type"))
		assert.Equal(t, testSubjectToken, form.Get("subject_token"))
		assert.Equal(t, testParticipantContext, form.Get("resource"))
		assert.Equal(t, testScope, form.Get("scope"))
		assert.Equal(t, testAudience, form.Get("audience"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokenResponse())
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithTokenExchangeAudience(testAudience),
		WithHttpClient(&http.Client{}),
	)

	token, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.NoError(t, err)
	assert.Equal(t, testAccessToken, token)
}

func TestGetToken_TokenFileNotFound(t *testing.T) {
	provider := NewTokenExchangeProvider("/nonexistent/path/token",
		WithTokenExchangeUrl("http://unused"),
		WithHttpClient(&http.Client{}),
	)

	token, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.Error(t, err)
	assert.Empty(t, token)
}

func TestGetToken_ServerReturns401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithHttpClient(&http.Client{}),
	)

	token, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.ErrorContains(t, err, "token exchange failed with status 401")
	assert.Empty(t, token)
}

func TestGetToken_ServerReturns500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithHttpClient(&http.Client{}),
	)

	token, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.ErrorContains(t, err, "token exchange failed with status 500")
	assert.Empty(t, token)
}

func TestGetToken_InvalidJsonResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithHttpClient(&http.Client{}),
	)

	token, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.ErrorContains(t, err, "error decoding token exchange response")
	assert.Empty(t, token)
}

func TestGetToken_HttpClientError(t *testing.T) {
	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl("http://127.0.0.1:1"), // nothing listening here
		WithHttpClient(&http.Client{}),
	)

	token, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.ErrorContains(t, err, "error sending token exchange request")
	assert.Empty(t, token)
}

func TestGetToken_InvalidUrl(t *testing.T) {
	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl("://bad url"),
		WithHttpClient(&http.Client{}),
	)

	token, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.ErrorContains(t, err, "error creating token exchange request")
	assert.Empty(t, token)
}

func TestGetToken_FormFieldsForwarded(t *testing.T) {
	var capturedForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedForm, _ = url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokenExchangeResponse{AccessToken: testAccessToken})
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithTokenExchangeAudience("my-audience"),
		WithHttpClient(&http.Client{}),
	)

	_, err := provider.GetToken(t.Context(), "my-scope", "did:web:alice")
	require.NoError(t, err)
	assert.Equal(t, "urn:ietf:params:oauth:grant-type:token-exchange", capturedForm.Get("grant_type"))
	assert.Equal(t, testSubjectToken, capturedForm.Get("subject_token"))
	assert.Equal(t, "did:web:alice", capturedForm.Get("resource"))
	assert.Equal(t, "my-scope", capturedForm.Get("scope"))
	assert.Equal(t, "my-audience", capturedForm.Get("audience"))
}

func TestGetToken_EmptyTokenFile(t *testing.T) {
	var capturedSubjectToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		capturedSubjectToken = form.Get("subject_token")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokenExchangeResponse{AccessToken: testAccessToken})
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, "")
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithHttpClient(&http.Client{}),
	)

	_, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.NoError(t, err)
	assert.Equal(t, "", capturedSubjectToken)
}

func TestGetToken_ContentTypeHeader(t *testing.T) {
	var capturedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokenExchangeResponse{AccessToken: testAccessToken})
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithHttpClient(&http.Client{}),
	)

	_, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(capturedContentType, "application/x-www-form-urlencoded"))
}

func TestNewTokenExchangeProvider_DefaultsNoopMonitor(t *testing.T) {
	provider := NewTokenExchangeProvider("/some/path")
	assert.NotNil(t, provider.monitor)
}

func TestGetToken_TokenAppendedToUrl(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokenExchangeResponse{AccessToken: testAccessToken})
	}))
	defer server.Close()

	tokenFile := writeTokenFile(t, testSubjectToken)
	provider := NewTokenExchangeProvider(tokenFile,
		WithTokenExchangeUrl(server.URL),
		WithHttpClient(&http.Client{}),
	)

	_, err := provider.GetToken(t.Context(), testScope, testParticipantContext)
	require.NoError(t, err)
	assert.Equal(t, "/token", capturedPath)
}
