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
			name:    "dataspace deployment without profile",
			payload: &NewDataspaceProfileDeployment{CellID: "cell1"},
			wantErr: true,
		},
		{
			name:    "dataspace deployment without cell",
			payload: &NewDataspaceProfileDeployment{ProfileID: "dp1"},
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
