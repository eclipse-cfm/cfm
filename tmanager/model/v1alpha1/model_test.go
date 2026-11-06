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

package v1alpha1

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/stretchr/testify/assert"
)

func TestRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		wantErr bool
	}{
		{
			name:    "cell",
			payload: &NewCell{State: "active", StateTimestamp: time.Now()},
		},
		{
			name:    "cell without state",
			payload: &NewCell{StateTimestamp: time.Now()},
			wantErr: true,
		},
		{
			name:    "cell without state timestamp",
			payload: &NewCell{State: "active"},
		},
		{
			name:    "participant deployment",
			payload: &NewParticipantProfileDeployment{Identifier: "did:web:example.com"},
		},
		{
			name:    "participant deployment without identifier",
			payload: &NewParticipantProfileDeployment{CellID: "cell1"},
			wantErr: true,
		},
		{
			name: "dataspace profile",
			payload: &NewDataspaceProfile{DataspaceSpec: DataspaceSpec{CredentialSpecs: []CredentialSpec{
				{Id: "c1", Type: "MembershipCredential", Issuer: "did:web:issuer", Format: "VC1_0_JWT"},
			}}},
		},
		{
			name: "dataspace profile with incomplete credential spec",
			payload: &NewDataspaceProfile{DataspaceSpec: DataspaceSpec{CredentialSpecs: []CredentialSpec{
				{Id: "c1", Type: "MembershipCredential", Issuer: "did:web:issuer"},
			}}},
			wantErr: true,
		},
		{
			name:    "dataspace deployment",
			payload: &NewDataspaceProfileDeployment{ProfileID: "dp1", CellID: "cell1"},
		},
		{
			name:    "dataspace deployment with non-UUID IDs",
			payload: &NewDataspaceProfileDeployment{ProfileID: "profiles/eu west", CellID: "urn:cell:1"},
		},
		{
			name:    "dataspace deployment without profile",
			payload: &NewDataspaceProfileDeployment{CellID: "cell1"},
			wantErr: true,
		},
		{
			name:    "dataspace deployment without cell",
			payload: &NewDataspaceProfileDeployment{ProfileID: "dp1"},
			wantErr: true,
		},
		{
			name: "participant deployment with all fields",
			payload: &NewParticipantProfileDeployment{
				Identifier:          "did:web:example.com:participants:acme",
				CellID:              "0b9c7f9e-1f7c-4e4b-9d43-6f7e0c1a2b3c",
				DataspaceProfileIDs: []string{"dp1"},
				ParticipantRoles:    map[string][]string{"dp1": {"member"}},
				VPAProperties:       map[string]map[string]any{"cfm.connector": {"replicas": 2}},
				Properties:          map[string]any{"region": "eu"},
			},
		},
		{
			name:    "participant deployment with overlong identifier",
			payload: &NewParticipantProfileDeployment{Identifier: "did:web:" + strings.Repeat("a", 505)},
			wantErr: true,
		},
		{
			name:    "participant deployment with overlong cell ID",
			payload: &NewParticipantProfileDeployment{Identifier: "acme", CellID: strings.Repeat("c", 257)},
			wantErr: true,
		},
		{
			name:    "participant deployment with empty dataspace profile ID",
			payload: &NewParticipantProfileDeployment{Identifier: "acme", DataspaceProfileIDs: []string{"dp1", ""}},
			wantErr: true,
		},
		{
			name:    "participant deployment with too many VPA properties",
			payload: &NewParticipantProfileDeployment{Identifier: "acme", VPAProperties: map[string]map[string]any{"cfm.connector": manyProperties(101)}},
			wantErr: true,
		},
		{
			name:    "participant deployment with overlong VPA property key",
			payload: &NewParticipantProfileDeployment{Identifier: "acme", VPAProperties: map[string]map[string]any{"cfm.connector": {strings.Repeat("k", 257): 1}}},
			wantErr: true,
		},
		{
			name:    "participant deployment with empty role",
			payload: &NewParticipantProfileDeployment{Identifier: "acme", ParticipantRoles: map[string][]string{"dp1": {""}}},
			wantErr: true,
		},
		{
			name: "nested property values",
			payload: &NewParticipantProfileDeployment{
				Identifier:    "acme",
				VPAProperties: map[string]map[string]any{"cfm.connector": nestedProperties()},
				Properties:    nestedProperties(),
			},
		},
		{
			name:    "tenant with nested property values",
			payload: &NewTenant{Properties: nestedProperties()},
		},
		{
			name:    "tenant with 100 properties",
			payload: &NewTenant{Properties: manyProperties(100)},
		},
		{
			name:    "tenant with too many properties",
			payload: &NewTenant{Properties: manyProperties(101)},
			wantErr: true,
		},
		{
			name:    "tenant with overlong property key",
			payload: &NewTenant{Properties: map[string]any{strings.Repeat("k", 257): 1}},
			wantErr: true,
		},
		{
			name:    "tenant patch with too many removals",
			payload: &TenantPropertiesDiff{Removed: make([]string, 101)},
			wantErr: true,
		},
		{
			name:    "dataspace profile with empty artifact",
			payload: &NewDataspaceProfile{Artifacts: []string{""}},
			wantErr: true,
		},
		{
			name:    "dataspace deployment with overlong cell ID",
			payload: &NewDataspaceProfileDeployment{ProfileID: "dp1", CellID: strings.Repeat("c", 257)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := model.Validator.Struct(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func manyProperties(n int) map[string]any {
	properties := make(map[string]any, n)
	for i := range n {
		properties[fmt.Sprintf("key%d", i)] = i
	}
	return properties
}

func nestedProperties() map[string]any {
	return map[string]any{
		"nested": map[string]any{"a": 1},
		"list":   []any{"x", 2},
		"nil":    nil,
	}
}
