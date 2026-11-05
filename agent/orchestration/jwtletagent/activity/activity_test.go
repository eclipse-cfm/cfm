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
	"sync"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
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

func TestProcessDispose_RejectsInvalidParticipantContextID(t *testing.T) {
	for _, value := range []any{"../other?force=true#", "", 42} {
		t.Run(fmt.Sprintf("%v", value), func(t *testing.T) {
			var deleted []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deleted = append(deleted, r.URL.Path)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			processor := newProcessorForTest(t, &fakeTokenProvider{token: stubToken()}, server.URL)

			ctx := newDeployContext()
			ctx.SetValue(participantContextIDKey, value)

			result := processor.ProcessDispose(ctx)

			require.EqualValues(t, api.ActivityResultFatalError, result.Result)
			assert.Empty(t, deleted)
		})
	}
}

// fakeJwtlet is an in-memory jwtlet management API that answers like the real one: creating a mapping that exists
// returns 409, and replacing or deleting a mapping that does not exist returns 404.
type fakeJwtlet struct {
	mu       sync.Mutex
	mappings map[string]resourceMapping
	requests []string
	// failCreate makes the create of a mapping for this client identifier fail with a 500
	failCreate string
}

func newFakeJwtlet(t *testing.T) (*fakeJwtlet, *httptest.Server) {
	j := &fakeJwtlet{mappings: map[string]resourceMapping{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		j.mu.Lock()
		defer j.mu.Unlock()
		j.requests = append(j.requests, r.Method)
		const prefix = "/api/v1/mappings/"
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mappings":
			var rm resourceMapping
			require.NoError(t, json.NewDecoder(r.Body).Decode(&rm))
			if rm.ClientIdentifier == j.failCreate {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			key := rm.ClientIdentifier + "/" + rm.ParticipantContext
			if _, exists := j.mappings[key]; exists {
				w.WriteHeader(http.StatusConflict)
				return
			}
			j.mappings[key] = rm
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, prefix):
			var rm resourceMapping
			require.NoError(t, json.NewDecoder(r.Body).Decode(&rm))
			key := strings.TrimPrefix(r.URL.Path, prefix)
			require.Equal(t, rm.ClientIdentifier+"/"+rm.ParticipantContext, key)
			if _, exists := j.mappings[key]; !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			j.mappings[key] = rm
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, prefix):
			key := strings.TrimPrefix(r.URL.Path, prefix)
			if _, exists := j.mappings[key]; !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(j.mappings, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	return j, server
}

func (j *fakeJwtlet) participantContexts() map[string]int {
	j.mu.Lock()
	defer j.mu.Unlock()
	contexts := map[string]int{}
	for _, rm := range j.mappings {
		contexts[rm.ParticipantContext]++
	}
	return contexts
}

func newDeployContextFor(orchestrationID string, processingData map[string]any) api.ActivityContext {
	data := map[string]any{"cfm.participant.id": "participant-123"}
	for key, value := range processingData {
		data[key] = value
	}
	return api.NewActivityContext(context.Background(), orchestrationID, api.Activity{ID: "jwtlet"}, data, map[string]any{})
}

// Running the deploy step again for the same orchestration, for example after a redelivery, must reuse the participant
// context of the first attempt and leave exactly one set of mappings behind.
func TestProcessDeploy_RepeatedAttemptsAreIdempotent(t *testing.T) {
	jwtlet, server := newFakeJwtlet(t)
	processor := newProcessorForTest(t, &fakeTokenProvider{token: stubToken()}, server.URL)

	first := newDeployContextFor("orch-1", nil)
	require.EqualValues(t, api.ActivityResultComplete, processor.ProcessDeploy(first).Result)
	second := newDeployContextFor("orch-1", nil)
	result := processor.ProcessDeploy(second)

	require.EqualValues(t, api.ActivityResultComplete, result.Result, "a repeated deploy must succeed, got error: %v", result.Error)
	firstID, _ := first.Value(participantContextIDKey)
	secondID, _ := second.Value(participantContextIDKey)
	assert.Equal(t, firstID, secondID)
	assert.Equal(t, firstID, second.OutputValues()[participantContextIDKey])
	assert.Equal(t, map[string]int{firstID.(string): 4}, jwtlet.participantContexts(), "exactly one set of mappings must exist")
	assert.Contains(t, jwtlet.requests, http.MethodPut, "existing mappings must be replaced")
}

func TestProcessDeploy_DerivesValidDistinctParticipantContextIDs(t *testing.T) {
	_, server := newFakeJwtlet(t)
	processor := newProcessorForTest(t, &fakeTokenProvider{token: stubToken()}, server.URL)

	first := newDeployContextFor("orch-1", nil)
	second := newDeployContextFor("orch-2", nil)
	require.EqualValues(t, api.ActivityResultComplete, processor.ProcessDeploy(first).Result)
	require.EqualValues(t, api.ActivityResultComplete, processor.ProcessDeploy(second).Result)

	firstID, _ := first.Value(participantContextIDKey)
	secondID, _ := second.Value(participantContextIDKey)
	assert.NotEqual(t, firstID, secondID)
	assert.Len(t, firstID, 32)
	assert.NoError(t, model.ValidateParticipantContextID(firstID.(string)))
}

// The participant context ID must not come from the orchestration payload, which would let a caller point the deploy at
// another participant's context.
func TestProcessDeploy_IgnoresParticipantContextIDFromPayload(t *testing.T) {
	jwtlet, server := newFakeJwtlet(t)
	processor := newProcessorForTest(t, &fakeTokenProvider{token: stubToken()}, server.URL)

	ctx := newDeployContextFor("orch-1", map[string]any{participantContextIDKey: "someone-elses-context"})
	require.EqualValues(t, api.ActivityResultComplete, processor.ProcessDeploy(ctx).Result)

	id, _ := ctx.Value(participantContextIDKey)
	assert.Equal(t, deriveParticipantContextID("orch-1", "jwtlet"), id)
	assert.NotContains(t, jwtlet.participantContexts(), "someone-elses-context")
}

// A deploy that fails part-way must still record the participant context ID, so the dispose step can delete the
// mappings that were created.
func TestProcessDeploy_FailedDeployCanBeDisposed(t *testing.T) {
	jwtlet, server := newFakeJwtlet(t)
	jwtlet.failCreate = "system:serviceaccount:test-ns:identityhub"
	processor := newProcessorForTest(t, &fakeTokenProvider{token: stubToken()}, server.URL)

	ctx := newDeployContextFor("orch-1", nil)
	result := processor.ProcessDeploy(ctx)
	require.EqualValues(t, api.ActivityResultFatalError, result.Result)
	require.ErrorContains(t, result.Error, "failed with status 500")
	require.NotEmpty(t, jwtlet.participantContexts(), "some mappings were created before the failure")

	_, ok := ctx.Value(participantContextIDKey)
	require.True(t, ok, "the participant context ID must be recorded before calling jwtlet")
	assert.NotContains(t, ctx.OutputValues(), participantContextIDKey, "a failed deploy must not report the ID as output")

	result = processor.ProcessDispose(ctx)
	require.EqualValues(t, api.ActivityResultComplete, result.Result, "dispose must clean up a partial deploy, got error: %v", result.Error)
	assert.Empty(t, jwtlet.participantContexts())
}

func TestProcessDispose_RepeatedAttemptsSucceed(t *testing.T) {
	jwtlet, server := newFakeJwtlet(t)
	processor := newProcessorForTest(t, &fakeTokenProvider{token: stubToken()}, server.URL)

	ctx := newDeployContextFor("orch-1", nil)
	require.EqualValues(t, api.ActivityResultComplete, processor.ProcessDeploy(ctx).Result)
	require.EqualValues(t, api.ActivityResultComplete, processor.ProcessDispose(ctx).Result)

	result := processor.ProcessDispose(ctx)

	require.EqualValues(t, api.ActivityResultComplete, result.Result, "a repeated dispose must succeed, got error: %v", result.Error)
	assert.Empty(t, jwtlet.participantContexts())
}

func TestProcessDispose_FailsOnServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	processor := newProcessorForTest(t, &fakeTokenProvider{token: stubToken()}, server.URL)

	ctx := newDeployContextFor("orch-1", map[string]any{participantContextIDKey: deriveParticipantContextID("orch-1", "jwtlet")})
	result := processor.ProcessDispose(ctx)

	require.EqualValues(t, api.ActivityResultFatalError, result.Result)
	assert.ErrorContains(t, result.Error, "failed with status 500")
}
