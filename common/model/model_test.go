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

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentManifest_JSONRoundTrip(t *testing.T) {
	originalManifest := OrchestrationManifest{
		ID:                "manifest-1",
		OrchestrationType: "test.orchestration",
		Payload: map[string]any{
			"environment": "production",
			"capacity":    100,
			"enabled":     true,
		},
	}

	jsonData, err := json.Marshal(originalManifest)
	require.NoError(t, err)

	var unmarshaledManifest OrchestrationManifest
	err = json.Unmarshal(jsonData, &unmarshaledManifest)
	require.NoError(t, err)

	assert.Equal(t, originalManifest.ID, unmarshaledManifest.ID)
	assert.Equal(t, originalManifest.OrchestrationType, unmarshaledManifest.OrchestrationType)

	require.NotNil(t, unmarshaledManifest.Payload)
	comparePayload(t, originalManifest.Payload, unmarshaledManifest.Payload)
}

func TestDeploymentResponse_JSONRoundTrip(t *testing.T) {
	originalResponse := OrchestrationResponse{
		ID:                "response-1",
		Success:           true,
		ErrorDetail:       "",
		ManifestID:        "manifest-1",
		OrchestrationType: "test.orchestration",
		Properties: map[string]any{
			"endpoint": "https://example.com",
			"status":   "running",
			"port":     8080,
		},
	}

	jsonData, err := json.Marshal(originalResponse)
	require.NoError(t, err)

	var unmarshaledResponse OrchestrationResponse
	err = json.Unmarshal(jsonData, &unmarshaledResponse)
	require.NoError(t, err)

	assert.Equal(t, originalResponse.ID, unmarshaledResponse.ID)
	assert.Equal(t, originalResponse.Success, unmarshaledResponse.Success)
	assert.Equal(t, originalResponse.ErrorDetail, unmarshaledResponse.ErrorDetail)
	assert.Equal(t, originalResponse.ManifestID, unmarshaledResponse.ManifestID)
	assert.Equal(t, originalResponse.OrchestrationType, unmarshaledResponse.OrchestrationType)

	require.NotNil(t, unmarshaledResponse.Properties)
	comparePayload(t, originalResponse.Properties, unmarshaledResponse.Properties)
}

// comparePayload is a helper function to compare payloads/properties accounting for JSON type conversions
func comparePayload(t *testing.T, original, unmarshaled map[string]any) {
	for key, originalValue := range original {
		unmarshaledValue, exists := unmarshaled[key]
		require.True(t, exists, "Key %s should exist in unmarshaled payload", key)

		switch v := originalValue.(type) {
		case int:
			assert.Equal(t, float64(v), unmarshaledValue, "Value for key %s should match", key)
		case []string:
			unmarshaledSlice, ok := unmarshaledValue.([]any)
			require.True(t, ok, "Value for key %s should be a slice", key)
			require.Len(t, unmarshaledSlice, len(v))
			for i, str := range v {
				assert.Equal(t, str, unmarshaledSlice[i])
			}
		case map[string]any:
			// Recursively compare nested maps
			unmarshaledMap, ok := unmarshaledValue.(map[string]any)
			require.True(t, ok, "Value for key %s should be a map", key)
			comparePayload(t, v, unmarshaledMap)
		default:
			assert.Equal(t, originalValue, unmarshaledValue, "Value for key %s should match", key)
		}
	}
}

func TestModelTypeValidation(t *testing.T) {
	type TypedStruct struct {
		Type string `validate:"required,modeltype"`
	}

	tests := []struct {
		name    string
		obj     TypedStruct
		wantErr bool
	}{
		{
			name:    "valid type",
			obj:     TypedStruct{Type: "valid-type"},
			wantErr: false,
		},
		{
			name:    "invalid type",
			obj:     TypedStruct{Type: "invalid@type"},
			wantErr: true,
		},
		{
			name:    "invalid # type",
			obj:     TypedStruct{Type: "invalid$type"},
			wantErr: true,
		},
		{
			name:    "empty type",
			obj:     TypedStruct{Type: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validator.Struct(tt.obj)
			if tt.wantErr {
				require.Error(t, err, "expected validation error")
			} else {
				require.NoError(t, err, "expected no validation error")
			}
		})
	}
}

func TestOrchestrationManifest_CompensatedManifestIDNotDeserialized(t *testing.T) {
	var manifest OrchestrationManifest
	err := json.Unmarshal([]byte(`{"id":"m1","correlationId":"c1","orchestrationType":"t","compensatedManifestId":"other"}`), &manifest)
	require.NoError(t, err)
	assert.Empty(t, manifest.CompensatedManifestID)
}

func TestOrchestrationResponse_CompensatedManifestIDRoundTrip(t *testing.T) {
	data, err := json.Marshal(OrchestrationResponse{ID: "r1", ManifestID: "m2", CompensatedManifestID: "m1"})
	require.NoError(t, err)
	var response OrchestrationResponse
	require.NoError(t, json.Unmarshal(data, &response))
	assert.Equal(t, "m1", response.CompensatedManifestID)
}

func TestManifestReplyToValidation(t *testing.T) {
	tests := []struct {
		name    string
		replyTo string
		wantErr bool
	}{
		{"empty", "", false},
		{"single token", "second-manager_response1", false},
		{"dot", "second.manager", true},
		{"wildcard", "*", true},
		{"full wildcard", ">", true},
		{"space", "second manager", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validator.Struct(OrchestrationManifest{ID: "id", CorrelationID: "corr", OrchestrationType: "type", ReplyTo: tt.replyTo})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, ValidationMessage(err), "replyTo")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestManifestNamespaceValidation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{"empty", "", false},
		{"single token", "second-manager_1", false},
		{"dot", "second.manager", true},
		{"wildcard", "*", true},
		{"full wildcard", ">", true},
		{"slash", "second/manager", true},
		{"space", "second manager", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validator.Struct(OrchestrationManifest{ID: "id", CorrelationID: "corr", OrchestrationType: "type", Namespace: tt.namespace})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, ValidationMessage(err), "namespace")
				assert.Error(t, ValidateNamespace(tt.namespace))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateNamespace_Empty(t *testing.T) {
	assert.Error(t, ValidateNamespace(""))
}

func TestNamespaceOrDefault(t *testing.T) {
	assert.Equal(t, DefaultNamespace, NamespaceOrDefault(""))
	assert.Equal(t, "other", NamespaceOrDefault("other"))
}

func TestManifestNamespace_JSON(t *testing.T) {
	data, err := json.Marshal(OrchestrationManifest{ID: "id", CorrelationID: "corr", OrchestrationType: "type"})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "namespace")

	var manifest OrchestrationManifest
	require.NoError(t, json.Unmarshal([]byte(`{"id":"id","namespace":"other"}`), &manifest))
	assert.Equal(t, "other", manifest.Namespace)
}
