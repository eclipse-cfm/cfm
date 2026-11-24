//  Copyright (c) 2026 Metaform Systems, Inc
//
//  This program and the accompanying materials are made available under the
//  terms of the Apache License, Version 2.0 which is available at
//  https://www.apache.org/licenses/LICENSE-2.0
//
//  SPDX-License-Identifier: Apache-2.0
//
//  Contributors:
//       Metaform Systems, Inc. - initial API and implementation
//

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testOrchestrationType is an orchestration type without meaning to the engine.
const testOrchestrationType model.OrchestrationType = "test.orchestration"

type fakeProvisionManager struct {
	api.ProvisionManager
	orchestration *api.Orchestration
	err           error
}

func (f *fakeProvisionManager) Start(context.Context, *model.OrchestrationManifest) (*api.Orchestration, error) {
	return f.orchestration, f.err
}

func (f *fakeProvisionManager) GetOrchestration(context.Context, string, string) (*api.Orchestration, error) {
	return f.orchestration, f.err
}

func newTestOrchestration() *api.Orchestration {
	return &api.Orchestration{
		ID:                "orchestration-1",
		CorrelationID:     "profile-1",
		OrchestrationType: testOrchestrationType,
		ProcessingData:    map[string]any{"participantContextId": "secret-context"},
		OutputData:        map[string]any{"holderPid": "holder-1"},
		EngineState:       api.EngineState{Error: "boom"},
	}
}

func decodeObject(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func createOrchestrationRequest() *http.Request {
	body := `{"id":"orchestration-1","correlationId":"profile-1","orchestrationType":"vpa.deploy"}`
	return httptest.NewRequest(http.MethodPost, "/orchestrations", strings.NewReader(body))
}

func TestOrchestrationData_OnlyForAdmins(t *testing.T) {
	requests := map[string]func() *http.Request{
		"get": func() *http.Request {
			return httptest.NewRequest(http.MethodGet, "/orchestrations/orchestration-1", nil)
		},
		"create": createOrchestrationRequest,
	}

	for name, newRequest := range requests {
		t.Run(name+" without the admin scope", func(t *testing.T) {
			pm := &fakeProvisionManager{orchestration: newTestOrchestration()}
			claims := &cfmauth.Claims{Subject: "client-1", Scopes: []string{scopePmRead, scopePmWrite}}
			router := newClaimsRouter(NewHandler(pm, nil, nil, system.NoopMonitor{}), claims)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, newRequest())

			require.Less(t, w.Code, 300, w.Body.String())
			body := decodeObject(t, w)
			assert.Equal(t, "orchestration-1", body["id"])
			assert.Empty(t, body["processingData"])
			assert.Empty(t, body["outputData"])
			assert.NotContains(t, body, "engineState")
			assert.NotContains(t, w.Body.String(), "secret-context")
		})

		t.Run(name+" with the admin scope", func(t *testing.T) {
			pm := &fakeProvisionManager{orchestration: newTestOrchestration()}
			claims := &cfmauth.Claims{Subject: "operator", Scopes: []string{scopePmRead, scopePmWrite, scopePmAdmin}}
			router := newClaimsRouter(NewHandler(pm, nil, nil, system.NoopMonitor{}), claims)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, newRequest())

			require.Less(t, w.Code, 300, w.Body.String())
			body := decodeObject(t, w)
			assert.Equal(t, map[string]any{"participantContextId": "secret-context"}, body["processingData"])
			assert.Equal(t, map[string]any{"holderPid": "holder-1"}, body["outputData"])
			assert.NotContains(t, body, "engineState")
		})
	}
}

func TestCreateOrchestration_IDConflict(t *testing.T) {
	pm := &fakeProvisionManager{err: fmt.Errorf("orchestration exists: %w", types.ErrConflict)}
	claims := &cfmauth.Claims{Subject: "client-1", Scopes: []string{scopePmWrite}}
	router := newClaimsRouter(NewHandler(pm, nil, nil, system.NoopMonitor{}), claims)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, createOrchestrationRequest())

	assert.Equal(t, http.StatusConflict, w.Code)
}

// fakeDefinitionManager records the orchestration definitions it is asked to create
type fakeDefinitionManager struct {
	api.DefinitionManager
	created []*api.OrchestrationDefinition
}

func (f *fakeDefinitionManager) CreateOrchestrationDefinition(_ context.Context, definition *api.OrchestrationDefinition) (*api.OrchestrationDefinition, error) {
	f.created = append(f.created, definition)
	return definition, nil
}

func createOrchestrationDefinitionRequest(compensation string) *http.Request {
	body := `{"id":"template-1","activities":{` +
		`"test.deploy":[{"id":"a1","type":"test-activity"}],` +
		`"test.rollback":[{"id":"a2","type":"test-activity"}]}` +
		compensation + `}`
	return httptest.NewRequest(http.MethodPost, "/orchestration-definitions", strings.NewReader(body))
}

func TestCreateOrchestrationDefinition_SetsCompensationType(t *testing.T) {
	dm := &fakeDefinitionManager{}
	claims := &cfmauth.Claims{Subject: "admin-1", Scopes: []string{scopePmAdmin}}
	router := newClaimsRouter(NewHandler(nil, dm, nil, system.NoopMonitor{}), claims)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, createOrchestrationDefinitionRequest(`,"compensation":{"test.deploy":"test.rollback"}`))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Len(t, dm.created, 2)
	for _, definition := range dm.created {
		if definition.Type == "test.deploy" {
			assert.Equal(t, model.OrchestrationType("test.rollback"), definition.CompensationType)
		} else {
			assert.Empty(t, definition.CompensationType)
		}
	}
}

func TestCreateOrchestrationDefinition_WithoutCompensation(t *testing.T) {
	dm := &fakeDefinitionManager{}
	claims := &cfmauth.Claims{Subject: "admin-1", Scopes: []string{scopePmAdmin}}
	router := newClaimsRouter(NewHandler(nil, dm, nil, system.NoopMonitor{}), claims)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, createOrchestrationDefinitionRequest(""))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Len(t, dm.created, 2)
	for _, definition := range dm.created {
		assert.Empty(t, definition.CompensationType)
	}
}

func TestCreateOrchestrationDefinition_RejectsInvalidCompensation(t *testing.T) {
	dm := &fakeDefinitionManager{}
	claims := &cfmauth.Claims{Subject: "admin-1", Scopes: []string{scopePmAdmin}}
	router := newClaimsRouter(NewHandler(nil, dm, nil, system.NoopMonitor{}), claims)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, createOrchestrationDefinitionRequest(`,"compensation":{"test.deploy":"test.unknown"}`))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, dm.created)
}
