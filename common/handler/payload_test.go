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
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payloadTestDto struct {
	Name       string            `json:"name"`
	Count      int               `json:"count"`
	Enabled    *bool             `json:"enabled"`
	Properties map[string]any    `json:"properties"`
	Tags       []string          `json:"tags"`
	Nested     payloadNestedDto  `json:"nested"`
	Labels     map[string]string `json:"labels"`
}

type payloadNestedDto struct {
	Value float64 `json:"value"`
}

func TestReadPayload_InvalidJSONMessages(t *testing.T) {
	tests := []struct {
		name string
		body string
		// message is a regular expression for the whole message
		message string
	}{
		{"syntax error", `{"name": }`, "Invalid JSON at offset 10"},
		{"string for an object", `{"properties": "x"}`, `Invalid JSON: field "properties" must be an object`},
		{"number for a string", `{"name": 1}`, `Invalid JSON: field "name" must be a string`},
		{"string for a number", `{"count": "1"}`, `Invalid JSON: field "count" must be a number`},
		{"string for a boolean pointer", `{"enabled": "yes"}`, `Invalid JSON: field "enabled" must be a boolean`},
		{"object for an array", `{"tags": {}}`, `Invalid JSON: field "tags" must be an array`},
		{"nested field", `{"nested": {"value": "x"}}`, `Invalid JSON: field "nested.value" must be a number`},
		// Go 1.27 includes the map key in the field path, earlier versions stop at the map
		{"map value", `{"labels": {"a": 1}}`, `Invalid JSON: field "labels(\.a)?" must be a string`},
		{"wrong top-level type", `[]`, "Invalid JSON"},
		{"truncated body", `{"name": "x"`, "Invalid JSON at offset 12"},
	}

	handler := HttpHandler{Monitor: system.NoopMonitor{}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newMockResponseWriter()
			req, err := http.NewRequest(http.MethodPost, "/test", strings.NewReader(tc.body))
			require.NoError(t, err)

			var payload payloadTestDto
			assert.False(t, handler.ReadPayload(w, req, &payload))
			assert.Equal(t, http.StatusBadRequest, w.statusCode)

			var response ErrorResponse
			require.NoError(t, json.Unmarshal(w.body.Bytes(), &response))
			assert.Regexp(t, "^"+tc.message+"$", response.Message)
			assert.NotContains(t, response.Message, "payloadTestDto")
			assert.NotContains(t, response.Message, "Go struct")
		})
	}
}
