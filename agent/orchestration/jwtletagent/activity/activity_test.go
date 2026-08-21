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
	"encoding/json"
	"fmt"
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

// newProcessorForTest builds a processor against the given mappings endpoint. Passing no
// clientMappings leaves Config.ClientMappings empty, which is what an unconfigured deployment looks
// like and therefore exercises the defaultClientMappings fallback.
func newProcessorForTest(t *testing.T, tp *fakeTokenProvider, mappingsBase string, clientMappings ...ClientMapping) *TokenExchangeActivityProcessor {
	t.Helper()
	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("workload-token"), 0o600))

	return NewProcessor(&Config{
		LogMonitor:              system.NoopMonitor{},
		TokenProvider:           tp,
		HttpClient:              http.DefaultClient,
		ManagementBasePath:      mappingsBase,
		TokenFilePath:           tokenFile,
		Audience:                "test-audience",
		ServiceAccountNamespace: "test-ns",
		ClientMappings:          clientMappings,
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

// TestProcessDeploy_UsesConfiguredNamespace asserts that the ServiceAccount client identifiers of
// the created resource mappings are derived from the configured namespace instead of a hardcoded one.
func TestProcessDeploy_UsesConfiguredNamespace(t *testing.T) {
	var identifiers []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rm resourceMapping
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rm))
		identifiers = append(identifiers, rm.ClientIdentifier)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tp := &fakeTokenProvider{token: stubToken()}
	processor := newProcessorForTest(t, tp, server.URL)

	result := processor.ProcessDeploy(newDeployContext())

	require.EqualValues(t, api.ActivityResultComplete, result.Result, "expected deploy to complete, got error: %v", result.Error)
	assert.Equal(t, []string{
		"system:serviceaccount:test-ns:cfm-agents",
		"system:serviceaccount:test-ns:controlplane",
		"system:serviceaccount:test-ns:identityhub",
		"system:serviceaccount:test-ns:siglet-sa",
	}, identifiers)
}

// TestProcessDeploy_MapsPerServiceAccountScopes asserts that each workload ServiceAccount gets the
// scopes its defaultClientMappings entry declares, rather than one scope set shared by all of them:
// every workload needs the vault read scope, and the control plane additionally needs the signaling
// scope to authorize DPS exchanges with the data plane.
func TestProcessDeploy_MapsPerServiceAccountScopes(t *testing.T) {
	scopesByIdentifier := map[string][]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rm resourceMapping
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rm))
		scopesByIdentifier[rm.ClientIdentifier] = rm.Scopes
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tp := &fakeTokenProvider{token: stubToken()}
	processor := newProcessorForTest(t, tp, server.URL)

	result := processor.ProcessDeploy(newDeployContext())

	require.EqualValues(t, api.ActivityResultComplete, result.Result, "expected deploy to complete, got error: %v", result.Error)
	assert.Equal(t, map[string][]string{
		"system:serviceaccount:test-ns:cfm-agents":   agentScopes,
		"system:serviceaccount:test-ns:controlplane": {"read", "signaling"},
		"system:serviceaccount:test-ns:identityhub":  {"read"},
		"system:serviceaccount:test-ns:siglet-sa":    {"read"},
	}, scopesByIdentifier)
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

// TestProcessDeploy_ConfiguredClientMappingsReplaceDefaults asserts that a configured client mapping
// list fully replaces the built-in defaults, so an operator can rename, re-scope or drop a built-in
// workload. The cfm-agents mapping is not part of that list and must still be created.
func TestProcessDeploy_ConfiguredClientMappingsReplaceDefaults(t *testing.T) {
	scopesByIdentifier := map[string][]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rm resourceMapping
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rm))
		scopesByIdentifier[rm.ClientIdentifier] = rm.Scopes
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tp := &fakeTokenProvider{token: stubToken()}
	processor := newProcessorForTest(t, tp, server.URL,
		ClientMapping{Name: "custom-cp", Scopes: []string{"read", "signaling"}},
		ClientMapping{Name: "custom-ih", Scopes: []string{"read"}},
	)

	result := processor.ProcessDeploy(newDeployContext())

	require.EqualValues(t, api.ActivityResultComplete, result.Result, "expected deploy to complete, got error: %v", result.Error)
	assert.Equal(t, map[string][]string{
		"system:serviceaccount:test-ns:cfm-agents": agentScopes,
		"system:serviceaccount:test-ns:custom-cp":  {"read", "signaling"},
		"system:serviceaccount:test-ns:custom-ih":  {"read"},
	}, scopesByIdentifier, "configured client mappings should replace the defaults entirely")
}

// TestProcessDispose_DeletesConfiguredClientMappings asserts that dispose tears down exactly the
// mappings deploy created, so a configured list does not leak resource mappings in jwtlet.
func TestProcessDispose_DeletesConfiguredClientMappings(t *testing.T) {
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tp := &fakeTokenProvider{token: stubToken()}
	processor := newProcessorForTest(t, tp, server.URL,
		ClientMapping{Name: "custom-cp", Scopes: []string{"read"}},
	)

	ctx := newDeployContext()
	require.EqualValues(t, api.ActivityResultComplete, processor.ProcessDeploy(ctx).Result)

	participantContextID, ok := ctx.Value(participantContextIDKey)
	require.True(t, ok)

	result := processor.ProcessDispose(ctx)

	require.EqualValues(t, api.ActivityResultComplete, result.Result, "expected dispose to complete, got error: %v", result.Error)
	assert.Equal(t, []string{
		fmt.Sprintf("/api/v1/mappings/system:serviceaccount:test-ns:cfm-agents/%v", participantContextID),
		fmt.Sprintf("/api/v1/mappings/system:serviceaccount:test-ns:custom-cp/%v", participantContextID),
	}, deleted)
}
