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

package activity

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTokenProvider records the scope it was asked for and returns a canned token/error.
type fakeTokenProvider struct {
	requestedScopes []string
	token           string
	err             error
}

func (f *fakeTokenProvider) GetToken(_ context.Context, scope string, _ string) (string, error) {
	f.requestedScopes = append(f.requestedScopes, scope)
	return f.token, f.err
}

// stubToken is a syntactically valid JWT (header.payload.signature) whose payload decodes to JSON,
// so decodeJWTClaims accepts it.
func stubToken() string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"participant-123"}`))
	return "header." + payload + ".signature"
}

func newProcessorForTest(t *testing.T, tp *fakeTokenProvider, mappingsBase string) *TokenExchangeActivityProcessor {
	t.Helper()
	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("workload-token"), 0o600))

	return NewProcessor(&Config{
		LogMonitor:         system.NoopMonitor{},
		TokenProvider:      tp,
		HttpClient:         http.DefaultClient,
		ManagementBasePath: mappingsBase,
		TokenFilePath:      tokenFile,
		Audience:           "test-audience",
	})
}

func newDeployContext() api.ActivityContext {
	processingData := map[string]any{"cfm.participant.id": "participant-123"}
	return api.NewActivityContext(context.Background(), "orch-1", api.Activity{}, processingData, map[string]any{})
}

// TestProcessDeploy_VerifiesFullAgentScopeSet asserts that the token-exchange verification step
// requests exactly the agentScopes set (space-joined), which is what proves every scope has a
// mapping seeded in jwtlet.
func TestProcessDeploy_VerifiesFullAgentScopeSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tp := &fakeTokenProvider{token: stubToken()}
	processor := newProcessorForTest(t, tp, server.URL)

	result := processor.ProcessDeploy(newDeployContext())

	require.EqualValues(t, api.ActivityResultComplete, result.Result, "expected deploy to complete, got error: %v", result.Error)
	require.Len(t, tp.requestedScopes, 1, "token exchange verification should request a token exactly once")
	assert.Equal(t, strings.Join(agentScopes, " "), tp.requestedScopes[0])
}

// TestProcessDeploy_FailsWhenScopeHasNoMapping asserts that if the token exchange fails — e.g.
// because a requested scope has no mapping seeded in jwtlet — deploy fails fast with a fatal error
// rather than completing and leaving the participant context broken for downstream agents.
func TestProcessDeploy_FailsWhenScopeHasNoMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tp := &fakeTokenProvider{err: assert.AnError}
	processor := newProcessorForTest(t, tp, server.URL)

	result := processor.ProcessDeploy(newDeployContext())

	require.EqualValues(t, api.ActivityResultFatalError, result.Result)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "token exchange")
	assert.Equal(t, strings.Join(agentScopes, " "), tp.requestedScopes[0])
}
