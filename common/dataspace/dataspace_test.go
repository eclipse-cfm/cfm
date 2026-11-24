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

package dataspace

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVPAManifest_JSONRoundTrip(t *testing.T) {
	originalManifest := VPAManifest{
		ID:      "vpa-manifest-1",
		VPAType: ConnectorType,
		CellID:  "cell-west-1",
		Properties: map[string]any{
			"region":    "us-west-1",
			"replicas":  3,
			"enabled":   true,
			"endpoints": []string{"http://api1.example.com", "http://api2.example.com"},
		},
	}

	jsonData, err := json.Marshal(originalManifest)
	require.NoError(t, err)

	var unmarshaledManifest VPAManifest
	err = json.Unmarshal(jsonData, &unmarshaledManifest)
	require.NoError(t, err)

	assert.Equal(t, originalManifest.ID, unmarshaledManifest.ID)
	assert.Equal(t, originalManifest.VPAType, unmarshaledManifest.VPAType)
	assert.Equal(t, originalManifest.CellID, unmarshaledManifest.CellID)

	// JSON unmarshaling converts numbers to float64 and slices to []any
	assert.Equal(t, map[string]any{
		"region":    "us-west-1",
		"replicas":  float64(3),
		"enabled":   true,
		"endpoints": []any{"http://api1.example.com", "http://api2.example.com"},
	}, unmarshaledManifest.Properties)
}

func TestValidateParticipantContextID(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"generated hex id", "0123456789abcdef0123456789abcdef", true},
		{"hyphenated id", "test-participant", true},
		{"underscore id", "pc_1", true},
		{"max length", strings.Repeat("a", 128), true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 129), false},
		{"path traversal", "../../other-participant", false},
		{"slash", "a/b", false},
		{"dot", "a.b", false},
		{"query and fragment", "x?force=true#", false},
		{"percent encoded", "x%2F..", false},
		{"whitespace", "a b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParticipantContextID(tt.id)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidator_ParticipantContextIDTag(t *testing.T) {
	type data struct {
		ParticipantContextID string `validate:"required,participantcontextid"`
	}
	require.NoError(t, model.Validator.Struct(data{ParticipantContextID: "test-participant"}))
	require.Error(t, model.Validator.Struct(data{ParticipantContextID: "../other"}))
	require.Error(t, model.Validator.Struct(data{}))
}
