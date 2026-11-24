//  Copyright (c) 2025 Metaform Systems, Inc
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
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namespaceRecorder records the namespace that each manager call is made for
type namespaceRecorder struct {
	api.ProvisionManager
	api.DefinitionManager
	namespaces []string
}

func (r *namespaceRecorder) record(namespace string) {
	r.namespaces = append(r.namespaces, namespace)
}

func (r *namespaceRecorder) Start(_ context.Context, manifest *model.OrchestrationManifest) (*api.Orchestration, error) {
	r.record(manifest.Namespace)
	return &api.Orchestration{ID: manifest.ID, Namespace: manifest.Namespace}, nil
}

func (r *namespaceRecorder) GetOrchestration(_ context.Context, namespace string, id string) (*api.Orchestration, error) {
	r.record(namespace)
	return &api.Orchestration{ID: id, Namespace: namespace}, nil
}

func (r *namespaceRecorder) QueryOrchestrations(_ context.Context, namespace string, _ query.Predicate, _ store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
	r.record(namespace)
	return func(func(*api.OrchestrationEntry, error) bool) {}
}

func (r *namespaceRecorder) CountOrchestrations(_ context.Context, namespace string, _ query.Predicate) (int64, error) {
	r.record(namespace)
	return 0, nil
}

func (r *namespaceRecorder) CreateOrchestrationDefinition(_ context.Context, definition *api.OrchestrationDefinition) (*api.OrchestrationDefinition, error) {
	r.record(definition.Namespace)
	return definition, nil
}

func (r *namespaceRecorder) DeleteOrchestrationDefinition(_ context.Context, namespace string, _ string) error {
	r.record(namespace)
	return nil
}

func (r *namespaceRecorder) GetOrchestrationDefinitions(_ context.Context, namespace string) ([]api.OrchestrationDefinition, error) {
	r.record(namespace)
	return nil, nil
}

func (r *namespaceRecorder) GetOrchestrationDefinitionsByTemplate(_ context.Context, namespace string, _ string) ([]api.OrchestrationDefinition, error) {
	r.record(namespace)
	return nil, nil
}

func (r *namespaceRecorder) CreateActivityDefinition(_ context.Context, definition *api.ActivityDefinition) (*api.ActivityDefinition, error) {
	r.record(definition.Namespace)
	return definition, nil
}

func (r *namespaceRecorder) DeleteActivityDefinition(_ context.Context, namespace string, _ api.ActivityType) error {
	r.record(namespace)
	return nil
}

func (r *namespaceRecorder) GetActivityDefinitions(_ context.Context, namespace string) ([]api.ActivityDefinition, error) {
	r.record(namespace)
	return nil, nil
}

type namespaceRequest struct {
	name   string
	method string
	path   string
	body   string
}

func namespaceRequests() []namespaceRequest {
	return []namespaceRequest{
		{"create orchestration", http.MethodPost, "/orchestrations", `{"id":"o1","correlationId":"c1","orchestrationType":"test.deploy"}`},
		{"get orchestration", http.MethodGet, "/orchestrations/o1", ""},
		{"query orchestrations", http.MethodPost, "/orchestrations/query", `{"predicate":"true"}`},
		{"create activity definition", http.MethodPost, "/activity-definitions", `{"type":"test-activity"}`},
		{"list activity definitions", http.MethodGet, "/activity-definitions", ""},
		{"delete activity definition", http.MethodDelete, "/activity-definitions/test-activity", ""},
		{"create orchestration definition", http.MethodPost, "/orchestration-definitions", `{"id":"template-1","activities":{"test.deploy":[{"id":"a1","type":"test-activity"}]}}`},
		{"list orchestration definitions", http.MethodGet, "/orchestration-definitions", ""},
		{"get orchestration definitions by template", http.MethodGet, "/orchestration-definitions/template-1", ""},
		{"delete orchestration definition", http.MethodDelete, "/orchestration-definitions/template-1", ""},
	}
}

func allScopes() []string {
	return []string{scopePmRead, scopePmWrite, scopePmAdmin}
}

// namespaceScopes returns every scope of each namespace
func namespaceScopes(namespaces ...string) []string {
	var scopes []string
	for _, namespace := range namespaces {
		for _, perm := range []permission{permissionRead, permissionWrite, permissionAdmin} {
			scopes = append(scopes, namespaceScope(namespace, perm))
		}
	}
	return scopes
}

// namespaceURL returns the path of the request on the routes of the namespace, or on the routes without a namespace if
// it is empty
func namespaceURL(namespace string, path string) string {
	if namespace == "" {
		return path
	}
	return "/namespaces/" + namespace + path
}

// serveNamespaceRequest serves the request on the routes of the namespace, or on the routes without a namespace if it
// is empty
func serveNamespaceRequest(t *testing.T, claims *cfmauth.Claims, request namespaceRequest, body string, namespace string) (*httptest.ResponseRecorder, *namespaceRecorder) {
	t.Helper()
	recorder := &namespaceRecorder{}
	router := newClaimsRouter(NewHandler(recorder, recorder, store.NoOpTransactionContext{}, system.NoopMonitor{}), claims)
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(request.method, namespaceURL(namespace, request.path), nil)
	} else {
		req = httptest.NewRequest(request.method, namespaceURL(namespace, request.path), strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w, recorder
}

func isPayloadRequest(request namespaceRequest) bool {
	return request.method == http.MethodPost && !strings.HasSuffix(request.path, "/query")
}

// withNamespace adds a namespace field to a JSON object
func withNamespace(body string, namespace string) string {
	return `{"namespace":"` + namespace + `",` + strings.TrimPrefix(body, "{")
}

// assertNamespaces asserts that the request succeeded and that every manager call was made for the namespace
func assertNamespaces(t *testing.T, w *httptest.ResponseRecorder, recorder *namespaceRecorder, namespace string) {
	t.Helper()
	require.Less(t, w.Code, 300, w.Body.String())
	require.NotEmpty(t, recorder.namespaces)
	for _, recorded := range recorder.namespaces {
		assert.Equal(t, namespace, recorded)
	}
}

func assertRejected(t *testing.T, w *httptest.ResponseRecorder, recorder *namespaceRecorder, status int) {
	t.Helper()
	assert.Equal(t, status, w.Code, w.Body.String())
	assert.Empty(t, recorder.namespaces)
}

func TestHandler_RoutesWithoutNamespaceUseDefaultNamespace(t *testing.T) {
	for name, scopes := range map[string][]string{
		"scopes without namespace":         allScopes(),
		"scopes of the default namespace":  namespaceScopes(model.DefaultNamespace),
		"scopes of several namespaces too": append(namespaceScopes("second", "third"), allScopes()...),
	} {
		claims := &cfmauth.Claims{Subject: "tmanager", Scopes: scopes}
		for _, request := range namespaceRequests() {
			t.Run(name+" "+request.name, func(t *testing.T) {
				w, recorder := serveNamespaceRequest(t, claims, request, request.body, "")
				assertNamespaces(t, w, recorder, model.DefaultNamespace)
			})
		}
	}
}

func TestHandler_URLSelectsNamespace(t *testing.T) {
	claims := &cfmauth.Claims{Subject: "operator", Scopes: append(namespaceScopes("second", "third"), allScopes()...)}
	for _, namespace := range []string{"second", "third", model.DefaultNamespace} {
		for _, request := range namespaceRequests() {
			t.Run(namespace+" "+request.name, func(t *testing.T) {
				w, recorder := serveNamespaceRequest(t, claims, request, request.body, namespace)
				assertNamespaces(t, w, recorder, namespace)
			})
		}
	}
}

func TestHandler_NamespaceWithoutScopeIsForbidden(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []string
		namespace string
	}{
		{"scopes of another namespace", namespaceScopes("second"), "third"},
		{"scopes without namespace", allScopes(), "second"},
		{"scopes of a namespace for the default namespace", namespaceScopes("second"), model.DefaultNamespace},
		{"scopes of a namespace for the routes without namespace", namespaceScopes("second"), ""},
		// scopes of other APIs and malformed scopes grant nothing
		{"other scopes", []string{"openid", "tenant-manager-api:read", "provision-manager-api:second:unknown", "provision-manager-api:a.b:read"}, "second"},
	}
	for _, tt := range tests {
		for _, request := range namespaceRequests() {
			t.Run(tt.name+" "+request.name, func(t *testing.T) {
				claims := &cfmauth.Claims{Subject: "client", Scopes: tt.scopes}
				w, recorder := serveNamespaceRequest(t, claims, request, request.body, tt.namespace)
				assertRejected(t, w, recorder, http.StatusForbidden)
			})
		}
	}
}

func TestHandler_PermissionIsCheckedPerNamespace(t *testing.T) {
	// the caller may read in the second namespace, and administer the third, which does not let it administer the second
	claims := &cfmauth.Claims{Subject: "client", Scopes: []string{namespaceScope("second", permissionRead), namespaceScope("third", permissionAdmin)}}

	w, recorder := serveNamespaceRequest(t, claims, namespaceRequest{method: http.MethodGet, path: "/activity-definitions"}, "", "second")
	assertNamespaces(t, w, recorder, "second")

	w, recorder = serveNamespaceRequest(t, claims, namespaceRequest{method: http.MethodPost, path: "/activity-definitions"}, `{"type":"test-activity"}`, "second")
	assertRejected(t, w, recorder, http.StatusForbidden)
	assert.Contains(t, w.Body.String(), "provision-manager-api:second:admin")

	w, recorder = serveNamespaceRequest(t, claims, namespaceRequest{method: http.MethodPost, path: "/activity-definitions"}, `{"type":"test-activity"}`, "third")
	assertNamespaces(t, w, recorder, "third")
}

func TestHandler_InvalidURLNamespace(t *testing.T) {
	claims := &cfmauth.Claims{Subject: "client", Scopes: namespaceScopes("second")}
	for _, request := range namespaceRequests() {
		t.Run(request.name, func(t *testing.T) {
			w, recorder := serveNamespaceRequest(t, claims, request, request.body, "second.other")
			assertRejected(t, w, recorder, http.StatusBadRequest)
		})
	}
}

func TestHandler_AnonymousCaller(t *testing.T) {
	for _, request := range namespaceRequests() {
		t.Run("default "+request.name, func(t *testing.T) {
			w, recorder := serveNamespaceRequest(t, cfmauth.AnonymousClaims(), request, request.body, "")
			assertNamespaces(t, w, recorder, model.DefaultNamespace)
		})
		t.Run("namespace "+request.name, func(t *testing.T) {
			w, recorder := serveNamespaceRequest(t, cfmauth.AnonymousClaims(), request, request.body, "second")
			assertNamespaces(t, w, recorder, "second")
		})
	}
}

func TestHandler_AcceptsPayloadWithOwnNamespace(t *testing.T) {
	claims := &cfmauth.Claims{Subject: "manager-2", Scopes: namespaceScopes("second")}
	for _, request := range namespaceRequests() {
		if !isPayloadRequest(request) {
			continue
		}
		t.Run(request.name, func(t *testing.T) {
			w, recorder := serveNamespaceRequest(t, claims, request, withNamespace(request.body, "second"), "second")
			assert.Equal(t, []string{"second"}, recorder.namespaces)
			assertNamespaces(t, w, recorder, "second")
		})
	}
}

func TestHandler_RejectsPayloadWithOtherNamespace(t *testing.T) {
	tests := []struct {
		name      string
		claims    *cfmauth.Claims
		namespace string
		other     string
	}{
		{"caller in a namespace", &cfmauth.Claims{Subject: "manager-2", Scopes: namespaceScopes("second")}, "second", model.DefaultNamespace},
		{"caller in the default namespace", &cfmauth.Claims{Subject: "tmanager", Scopes: allScopes()}, "", "second"},
		{"caller with several namespaces", &cfmauth.Claims{Subject: "operator", Scopes: namespaceScopes("second", "third")}, "third", "second"},
		{"anonymous caller", cfmauth.AnonymousClaims(), "", "second"},
	}
	for _, tt := range tests {
		for _, request := range namespaceRequests() {
			if !isPayloadRequest(request) {
				continue
			}
			t.Run(tt.name+" "+request.name, func(t *testing.T) {
				w, recorder := serveNamespaceRequest(t, tt.claims, request, withNamespace(request.body, tt.other), tt.namespace)
				assertRejected(t, w, recorder, http.StatusForbidden)
			})
		}
	}
}

func TestHandler_RejectsInvalidPayloadNamespace(t *testing.T) {
	claims := &cfmauth.Claims{Subject: "manager-2", Scopes: namespaceScopes("second")}
	for _, request := range namespaceRequests() {
		if !isPayloadRequest(request) {
			continue
		}
		t.Run(request.name, func(t *testing.T) {
			w, recorder := serveNamespaceRequest(t, claims, request, withNamespace(request.body, "second.other"), "second")
			assertRejected(t, w, recorder, http.StatusBadRequest)
		})
	}
}

func TestHandler_QueryLinksKeepNamespace(t *testing.T) {
	claims := &cfmauth.Claims{Subject: "manager-2", Scopes: namespaceScopes("second")}
	w, _ := serveNamespaceRequest(t, claims, namespaceRequest{method: http.MethodPost, path: "/orchestrations/query"}, `{"predicate":"true"}`, "second")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Link"), "</namespaces/second/orchestrations/query?")
}

func TestOrchestrationData_OnlyForAdminsOfNamespace(t *testing.T) {
	newRequest := func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/namespaces/second/orchestrations/orchestration-1", nil)
	}
	for name, scopes := range map[string][]string{
		"admin of another namespace": {namespaceScope("second", permissionRead), namespaceScope("third", permissionAdmin)},
		"admin without namespace":    {namespaceScope("second", permissionRead), scopePmAdmin},
	} {
		t.Run(name, func(t *testing.T) {
			pm := &fakeProvisionManager{orchestration: newTestOrchestration()}
			router := newClaimsRouter(NewHandler(pm, nil, nil, system.NoopMonitor{}), &cfmauth.Claims{Subject: "client", Scopes: scopes})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, newRequest())

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Empty(t, decodeObject(t, w)["processingData"])
		})
	}

	pm := &fakeProvisionManager{orchestration: newTestOrchestration()}
	claims := &cfmauth.Claims{Subject: "client", Scopes: []string{namespaceScope("second", permissionRead), namespaceScope("second", permissionAdmin)}}
	router := newClaimsRouter(NewHandler(pm, nil, nil, system.NoopMonitor{}), claims)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, newRequest())
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NotEmpty(t, decodeObject(t, w)["processingData"])
}
