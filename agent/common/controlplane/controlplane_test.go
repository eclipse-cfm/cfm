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

package controlplane

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// shrinkPatchBackoffs makes the patch-verification retries near-instant for the duration of a test.
func shrinkPatchBackoffs(t *testing.T) {
	original := patchConfigBackoffs
	patchConfigBackoffs = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { patchConfigBackoffs = original })
}

// configFake is a stateful stand-in for the control plane's participant-config endpoints: PATCH
// merges into the stored entries (unless told to lose the write), GET serves them back. Private
// entries are served with scrambled values, like the real control plane returns them encrypted.
type configFake struct {
	mu             sync.Mutex
	entries        map[string]string
	privateEntries map[string]string
	patchCount     int
	// losePatches drops the effect (but not the 204) of the first N PATCH requests — the
	// lost-update behavior observed under concurrent merges.
	losePatches int
}

func newConfigFake(losePatches int) *configFake {
	return &configFake{entries: map[string]string{}, privateEntries: map[string]string{}, losePatches: losePatches}
}

func (f *configFake) handler(t *testing.T, participant string) http.HandlerFunc {
	path := CreateParticipantURL + "/" + participant + "/config"
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == path && r.Method == http.MethodPatch:
			f.patchCount++
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var data map[string]any
			require.NoError(t, json.Unmarshal(body, &data))
			if f.patchCount > f.losePatches {
				merge(f.entries, data["entries"])
				merge(f.privateEntries, data["privateEntries"])
			}
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == path && r.Method == http.MethodGet:
			scrambled := map[string]string{}
			for k := range f.privateEntries {
				scrambled[k] = "encrypted:" + k
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"participantContextId": participant,
				"entries":              f.entries,
				"privateEntries":       scrambled,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func merge(target map[string]string, raw any) {
	if object, ok := raw.(map[string]any); ok {
		for k, v := range object {
			if s, ok := v.(string); ok {
				target[k] = s
			}
		}
	}
}

func TestParticipantContextConfig_SerDes(t *testing.T) {
	orig := ParticipantContextConfig{
		ParticipantContextID: "pc-1",
		Entries:              map[string]string{"k": "v"},
		SecretEntries:        map[string]string{"s": "secret"},
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify JSON keys and values via generic map
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	if id, ok := m["participantContextId"].(string); !ok || id != orig.ParticipantContextID {
		t.Fatalf("unexpected participantContextId: %v", m["participantContextId"])
	}

	entries, ok := m["entries"].(map[string]any)
	if !ok || entries["k"] != "v" {
		t.Fatalf("unexpected entries: %#v", m["entries"])
	}

	privateEntries, ok := m["privateEntries"].(map[string]any)
	if !ok || privateEntries["s"] != "secret" {
		t.Fatalf("unexpected privateEntries: %#v", m["privateEntries"])
	}

	// Round-trip into struct
	var decoded ParticipantContextConfig
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal to struct failed: %v", err)
	}

	if !reflect.DeepEqual(orig, decoded) {
		t.Fatalf("round-trip mismatch\ngot:  %#v\nwant: %#v", decoded, orig)
	}
}

func TestParticipantContext_SerDes(t *testing.T) {
	orig := ParticipantContext{
		ParticipantContextID: "pc-2",
		Identifier:           "did:example:123",
		Properties: map[string]any{
			"name":   "alice",
			"age":    float64(30), // use float64 so JSON round-trip preserves type when decoding into interface{}
			"active": true,
			"nested": map[string]any{"x": "y"},
		},
		State: ParticipantContextStateActivated,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify JSON keys and numeric state via generic map (numbers become float64)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	if id, ok := m["id"].(string); !ok || id != orig.ParticipantContextID {
		t.Fatalf("unexpected id: %v", m["id"])
	}

	if identity, ok := m["identity"].(string); !ok || identity != orig.Identifier {
		t.Fatalf("unexpected identity: %v", m["identity"])
	}

	if st, ok := m["state"].(string); !ok || st != string(orig.State) {
		t.Fatalf("unexpected state: %#v", m["state"])
	}

	// Round-trip into struct
	var decoded ParticipantContext
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal to struct failed: %v", err)
	}

	// Compare top-level fields
	if decoded.ParticipantContextID != orig.ParticipantContextID || decoded.Identifier != orig.Identifier || decoded.State != orig.State {
		t.Fatalf("round-trip top-level mismatch\ngot:  %#v\nwant: %#v", decoded, orig)
	}

	// Deep compare properties
	if !reflect.DeepEqual(decoded.Properties, orig.Properties) {
		t.Fatalf("properties mismatch\ngot:  %#v\nwant: %#v", decoded.Properties, orig.Properties)
	}
}

func TestCreateParticipant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == CreateParticipantURL && r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var data map[string]any
			err = json.Unmarshal(body, &data)

			require.Equal(t, "test-participant", data["@id"])
			require.Equal(t, "did:web:test-participant", data["identity"])
			require.Emptyf(t, data["properties"], "expected empty properties map")
			require.NotNil(t, data["state"])

			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContext{
		ParticipantContextID: "test-participant",
		Identifier:           "did:web:test-participant",
		Properties:           make(map[string]any),
		State:                ParticipantContextStateActivated,
	}

	err := client.CreateParticipantContext(t.Context(), context)
	require.NoError(t, err)
}

func TestCreateParticipant_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test error"))
	client := HttpManagementAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContext{
		ParticipantContextID: "test-participant",
		Identifier:           "did:web:test-participant",
		Properties:           make(map[string]any),
		State:                ParticipantContextStateActivated,
	}

	require.ErrorContains(t, client.CreateParticipantContext(t.Context(), context), "test error")
}

func TestCreateParticipant_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContext{
		ParticipantContextID: "test-participant",
		Identifier:           "did:web:test-participant",
		Properties:           make(map[string]any),
		State:                ParticipantContextStateActivated,
	}

	require.ErrorContains(t, client.CreateParticipantContext(t.Context(), context), "received status code 400")
}

func TestCreateParticipant_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContext{
		ParticipantContextID: "test-participant",
		Identifier:           "did:web:test-participant",
		Properties:           make(map[string]any),
		State:                ParticipantContextStateActivated,
	}

	require.ErrorContains(t, client.CreateParticipantContext(t.Context(), context), "received status code 409")
}

func TestAssociateProfiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == CreateParticipantURL+"/test-participant/profiles" && r.Method == http.MethodPut {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var data map[string]any
			require.NoError(t, json.Unmarshal(body, &data))

			require.Equal(t, dataspaceProfileType, data["@type"])
			require.Equal(t, []any{"cx-neptune", "cx-pluto"}, data["profiles"])

			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.AssociateProfiles(t.Context(), "test-participant", []string{"cx-neptune", "cx-pluto"})
	require.NoError(t, err)
}

func TestAssociateProfiles_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test error"))
	client := HttpManagementAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.AssociateProfiles(t.Context(), "test-participant", []string{"cx-neptune"}), "test error")
}

func TestAssociateProfiles_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.AssociateProfiles(t.Context(), "test-participant", []string{"cx-neptune"}), "received status code 400")
}

func TestDeleteParticipant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == CreateParticipantURL+"/test-participant" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.DeleteParticipantContext(t.Context(), "test-participant")
	require.NoError(t, err)
}

func TestDeleteParticipant_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test error"))
	client := HttpManagementAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.DeleteParticipantContext(t.Context(), "test-participant"), "test error")
}

func TestDeleteParticipant_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("participant not found"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.DeleteParticipantContext(t.Context(), "test-participant"), "not found in control plane")
}

func TestDeleteParticipant_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.DeleteParticipantContext(t.Context(), "test-participant"), "received status code 500")
}

func TestPatchConfig(t *testing.T) {
	fake := newConfigFake(0)
	server := httptest.NewServer(fake.handler(t, "test-participant"))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	config := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		Entries: map[string]string{
			"edc.iam.sts.type":              "signature",
			"edc.iam.sts.signature.keyname": "priv-key-alias",
			"edc.iam.sts.signature.kid":     "key-1",
		},
	}

	require.NoError(t, client.PatchConfig(t.Context(), "test-participant", config))

	// applied, verified, and no retry was needed
	require.Equal(t, 1, fake.patchCount)
	require.Equal(t, "signature", fake.entries["edc.iam.sts.type"])
	require.Equal(t, "priv-key-alias", fake.entries["edc.iam.sts.signature.keyname"])
	require.Equal(t, "key-1", fake.entries["edc.iam.sts.signature.kid"])
}

func TestPatchConfig_RepatchesALostWrite(t *testing.T) {
	// The lost-update defense: the control plane merge has been observed to drop one writer's
	// entries under concurrency while answering 2xx. The verification read must detect the loss
	// and re-patch until the entries are visible.
	shrinkPatchBackoffs(t)
	fake := newConfigFake(1)
	server := httptest.NewServer(fake.handler(t, "test-participant"))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	config := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		Entries:              map[string]string{"edc.iam.sts.type": "signature"},
		SecretEntries:        map[string]string{"edc.vault.hashicorp.config": "{}"},
	}

	require.NoError(t, client.PatchConfig(t.Context(), "test-participant", config))
	require.Equal(t, 2, fake.patchCount)
	require.Equal(t, "signature", fake.entries["edc.iam.sts.type"])
}

func TestPatchConfig_FailsWhenTheWriteNeverSticks(t *testing.T) {
	// A write that never becomes visible must surface as an ERROR, never as silent success — a
	// participant context without its config entries is permanently broken.
	shrinkPatchBackoffs(t)
	fake := newConfigFake(1000)
	server := httptest.NewServer(fake.handler(t, "test-participant"))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	config := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		Entries:              map[string]string{"edc.iam.sts.type": "signature"},
	}

	err := client.PatchConfig(t.Context(), "test-participant", config)
	require.ErrorContains(t, err, "not visible")
	require.ErrorContains(t, err, "entries[edc.iam.sts.type]")
	require.Equal(t, len(patchConfigBackoffs)+1, fake.patchCount)
}

func TestPatchConfig_VerifiesPrivateEntriesByKeyOnly(t *testing.T) {
	// The control plane returns private entries encrypted; their values cannot be compared, only
	// their presence — an encrypted value must not be mistaken for a lost write.
	fake := newConfigFake(0)
	server := httptest.NewServer(fake.handler(t, "test-participant"))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	config := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		SecretEntries:        map[string]string{"edc.vault.hashicorp.config": `{"vault":"config"}`},
	}

	require.NoError(t, client.PatchConfig(t.Context(), "test-participant", config))
	require.Equal(t, 1, fake.patchCount)
}

func TestGetConfig_ParsesPlainAndJsonLdShapes(t *testing.T) {
	responses := []string{
		// plain terms, plain objects
		`{"participantContextId":"p1","entries":{"k1":"v1"},"privateEntries":{"s1":"enc"}}`,
		// expanded IRIs with JSON-literal wrappers
		`{"@type":"ParticipantContextConfig",
		  "https://w3id.org/edc/v0.0.1/ns/entries":{"@value":{"k1":"v1"},"@type":"@json"},
		  "https://w3id.org/edc/v0.0.1/ns/privateEntries":{"@value":{"s1":"enc"},"@type":"@json"}}`,
	}
	for i, response := range responses {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(response))
		}))

		tp := mocks.NewMockTokenProvider(t)
		tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
		client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

		config, err := client.GetConfig(t.Context(), "p1")
		require.NoError(t, err, "response shape %d", i)
		require.Equal(t, map[string]string{"k1": "v1"}, config.Entries, "response shape %d", i)
		require.Equal(t, map[string]string{"s1": "enc"}, config.SecretEntries, "response shape %d", i)
		server.Close()
	}
}

func TestPatchConfig_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test error"))
	client := HttpManagementAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.PatchConfig(t.Context(), "test-participant", ParticipantContextConfig{ParticipantContextID: "test-participant"})
	require.ErrorContains(t, err, "test error")
}

func TestPatchConfig_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.PatchConfig(t.Context(), "test-participant", ParticipantContextConfig{ParticipantContextID: "test-participant"})
	require.ErrorContains(t, err, "received status code 500")
}

func TestRegisterDataPlane(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == CreateParticipantURL+"/test-participant/dataplanes" {
			require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	err := client.RegisterDataPlane(t.Context(), "test-participant", DataPlaneRegistration{
		ID:            "test-participant-siglet",
		TransferTypes: []string{"HttpData-PULL"},
		Endpoint:      "http://siglet.edc-v.svc.cluster.local:8081/api/v1/test-participant/dataflows",
	})
	require.NoError(t, err)
	require.Equal(t, "test-participant-siglet", received["dataplaneId"])
	require.Equal(t, "http://siglet.edc-v.svc.cluster.local:8081/api/v1/test-participant/dataflows", received["endpoint"])
	require.NotContains(t, received, "authorization", "an absent authorization profile must not be sent")
}

func TestRegisterDataPlane_WithAuthorization(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == CreateParticipantURL+"/test-participant/dataplanes" {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &received))
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	err := client.RegisterDataPlane(t.Context(), "test-participant", DataPlaneRegistration{
		ID:            "test-participant-siglet",
		TransferTypes: []string{"HttpData-PULL"},
		Endpoint:      "http://siglet.edc-v.svc.cluster.local:8081/api/v1/test-participant/dataflows",
		Authorization: map[string]any{
			"type":                  "oauth2_token_exchange",
			"tokenExchangeEndpoint": "https://broker.example.com/token",
			"issuer":                "https://broker.example.com",
			"jwksUri":               "https://broker.example.com/.well-known/jwks.json",
			"resource":              "test-participant",
		},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"type":                  "oauth2_token_exchange",
		"tokenExchangeEndpoint": "https://broker.example.com/token",
		"issuer":                "https://broker.example.com",
		"jwksUri":               "https://broker.example.com/.well-known/jwks.json",
		"resource":              "test-participant",
	}, received["authorization"])
}

func TestRegisterDataPlane_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test error"))
	client := HttpManagementAPIClient{BaseURL: "http://foo.bar", TokenProvider: tp, HttpClient: &http.Client{}}

	err := client.RegisterDataPlane(t.Context(), "test-participant", DataPlaneRegistration{ID: "test-participant-siglet"})
	require.ErrorContains(t, err, "test error")
}

func TestRegisterDataPlane_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	err := client.RegisterDataPlane(t.Context(), "test-participant", DataPlaneRegistration{ID: "test-participant-siglet"})
	require.ErrorContains(t, err, "received status code 500")
}

func TestUnregisterDataPlane(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == CreateParticipantURL+"/test-participant/dataplanes/siglet-1" {
			require.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	err := client.UnregisterDataPlane(t.Context(), "test-participant", "siglet-1")
	require.NoError(t, err)
}

func TestUnregisterDataPlane_NotFoundIsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	err := client.UnregisterDataPlane(t.Context(), "test-participant", "siglet-1")
	require.NoError(t, err)
}

func TestUnregisterDataPlane_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken", mock.Anything, mock.Anything, mock.Anything).Return("token", nil)
	client := HttpManagementAPIClient{BaseURL: server.URL, TokenProvider: tp, HttpClient: &http.Client{}}

	err := client.UnregisterDataPlane(t.Context(), "test-participant", "siglet-1")
	require.ErrorContains(t, err, "received status code 500")
}
