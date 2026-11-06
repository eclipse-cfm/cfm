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

type fakeProvisionManager struct {
	api.ProvisionManager
	orchestration *api.Orchestration
	err           error
}

func (f *fakeProvisionManager) Start(context.Context, *model.OrchestrationManifest) (*api.Orchestration, error) {
	return f.orchestration, f.err
}

func (f *fakeProvisionManager) GetOrchestration(context.Context, string) (*api.Orchestration, error) {
	return f.orchestration, f.err
}

func newTestOrchestration() *api.Orchestration {
	return &api.Orchestration{
		ID:                "orchestration-1",
		CorrelationID:     "profile-1",
		OrchestrationType: model.VPADeployType,
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
