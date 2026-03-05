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
	"testing"

	"github.com/metaform/connector-fabric-manager/common/mocks"
	"github.com/stretchr/testify/require"
)

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
	tp.On("GetToken").Return("token", nil)
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

	err := client.CreateParticipantContext(context)
	require.NoError(t, err)
}

func TestCreateParticipant_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("", fmt.Errorf("test error"))
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

	require.ErrorContains(t, client.CreateParticipantContext(context), "test error")
}

func TestCreateParticipant_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("test token", nil)
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

	require.ErrorContains(t, client.CreateParticipantContext(context), "received status code 400")
}

func TestCreateParticipant_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("test token", nil)
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

	require.ErrorContains(t, client.CreateParticipantContext(context), "received status code 409")
}

func TestCreateParticipantConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == CreateParticipantURL+"/test-participant/config" && r.Method == http.MethodPut {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var data map[string]any
			err = json.Unmarshal(body, &data)

			require.Equal(t, "test-participant", data["identity"])
			require.NotEmptyf(t, data["entries"], "expected empty properties map")
			require.NotEmptyf(t, data["privateEntries"], "expected empty properties map")

			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		Entries:              map[string]string{"foo": "bar"},
		SecretEntries:        map[string]string{"secret-foo": "secret-bar"},
	}

	err := client.CreateConfig("test-participant", context)
	require.NoError(t, err)
}

func TestCreateParticipantConfig_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("", fmt.Errorf("test error"))
	client := HttpManagementAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		Entries:              map[string]string{"foo": "bar"},
		SecretEntries:        map[string]string{"secret-foo": "secret-bar"},
	}

	require.ErrorContains(t, client.CreateConfig("test-participant", context), "test error")
}

func TestCreateParticipantConfig_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		Entries:              map[string]string{"foo": "bar"},
		SecretEntries:        map[string]string{"secret-foo": "secret-bar"},
	}

	require.ErrorContains(t, client.CreateConfig("test-participant", context), "foobar")
}

func TestCreateParticipantConfig_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("foobar"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	context := ParticipantContextConfig{
		ParticipantContextID: "test-participant",
		Entries:              map[string]string{"foo": "bar"},
		SecretEntries:        map[string]string{"secret-foo": "secret-bar"},
	}

	require.ErrorContains(t, client.CreateConfig("test-participant", context), "foobar")
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
	tp.On("GetToken").Return("token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	err := client.DeleteParticipantContext("test-participant")
	require.NoError(t, err)
}

func TestDeleteParticipant_AuthError(t *testing.T) {
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("", fmt.Errorf("test error"))
	client := HttpManagementAPIClient{
		BaseURL:       "http://foo.bar",
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.DeleteParticipantContext("test-participant"), "test error")
}

func TestDeleteParticipant_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("participant not found"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.DeleteParticipantContext("test-participant"), "not found in control plane")
}

func TestDeleteParticipant_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()
	tp := mocks.NewMockTokenProvider(t)
	tp.On("GetToken").Return("test token", nil)
	client := HttpManagementAPIClient{
		BaseURL:       server.URL,
		TokenProvider: tp,
		HttpClient:    &http.Client{},
	}

	require.ErrorContains(t, client.DeleteParticipantContext("test-participant"), "received status code 500")
}
