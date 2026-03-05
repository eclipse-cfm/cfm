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

package identityhub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewParticipantManifest_WithDefaults(t *testing.T) {
	manifest := NewParticipantManifest("test-id", "did:web:foo", "http://example.com/credentialservice", "http://example.com/protocol")

	require.Equal(t, manifest.CredentialServiceID, "test-id-credentialservice")
	require.Equal(t, manifest.ProtocolServiceID, "test-id-dsp")
	require.Equal(t, manifest.IsActive, true)
	require.Equal(t, manifest.KeyGeneratorParameters.KeyID, "did:web:foo#"+DefaultKeyID)
	require.Equal(t, manifest.KeyGeneratorParameters.PrivateKeyAlias, "did:web:foo#"+DefaultKeyID)
	require.Equal(t, manifest.VaultConfig.SecretPath, "v1/participants")
	require.Equal(t, manifest.VaultConfig.FolderPath, "test-id/identityhub")
}

func TestParticipantManifest_Serialization(t *testing.T) {
	orig := NewParticipantManifest("test-id", "did:web:foo", "http://example.com/credentialservice", "http://example.com/protocol")

	data, err := json.Marshal(orig)
	require.NoError(t, err)

	var got ParticipantManifest
	require.NoError(t, json.Unmarshal(data, &got))

	require.Equal(t, orig, got)
}

func TestCredentialRequest_Serialization(t *testing.T) {
	cr := CredentialRequest{
		IssuerDID: "did:example:issuer",
		HolderPID: "holder-123",
		Credentials: []CredentialType{
			{
				Format:                 "ldp",
				Type:                   "VerifiableCredential",
				CredentialDefinitionID: "cred-1",
			},
		},
	}

	data, err := json.Marshal(cr)
	require.NoError(t, err)

	var out CredentialRequest
	require.NoError(t, json.Unmarshal(data, &out))

	require.Equal(t, cr, out)
}

func TestCredentialRequest_Deserialization(t *testing.T) {
	jsonString := `{"issuerDid": "did:example:issuer", "holderPid": "holder-123", "credentials": [{"format": "ldp", "type": "VerifiableCredential", "id": "cred-1"}]}`

	request := CredentialRequest{}
	err := json.Unmarshal([]byte(jsonString), &request)
	require.NoError(t, err)
	require.Equal(t, "did:example:issuer", request.IssuerDID)
	require.Equal(t, "holder-123", request.HolderPID)
	require.Len(t, request.Credentials, 1)
}
