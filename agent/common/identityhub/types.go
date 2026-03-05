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
	"strings"

	"github.com/metaform/connector-fabric-manager/agent/edcv"
)

const (
	DefaultKeyID     = "key1"
	DefaultAlgorithm = "EDDSA"
	DefaultCurve     = "Ed25519"
)

type KeyGeneratorParameters struct {

	// KeyID specifies the unique identifier for a cryptographic key within the key generation parameters. This will be prefixed with the holder's DID
	KeyID string
	// PrivateKeyAlias specifies the alias under which the Key will be stored in the participant's vault partition
	PrivateKeyAlias string
	// KeyAlgorithm specifies the algorithm used to generate the key, for example, "EC" or "EDDSA"
	KeyAlgorithm string
	//Curve specifies the curve used to generate the key, for example, "P-256" or "Ed25519"
	Curve string
}

// ParticipantManifest defines parameters and configuration values for creating a participant context in IdentityHub
type ParticipantManifest struct {
	// CredentialServiceURL the URL of the credential service, i.e. the query and storage endpoints of IdentityHub
	CredentialServiceURL string
	// CredentialServiceID the ID of the credential service. Defaults to "<PARTICIPANT_CONTEXT_ID>-credential-service"
	CredentialServiceID string
	// ProtocolServiceURL the URL of the protocol service, i.e. the DSP protocol endpoint of the control plane
	ProtocolServiceURL string
	// ProtocolServiceID the ID of the protocol service. Defaults to "<PARTICIPANT_CONTEXT_ID>-dsp"
	ProtocolServiceID string
	// IsActive indicates whether the participant is set to "active" after creation. Defaults to true
	IsActive bool
	// ParticipantContextID the unique identifier of the participant context. This is NOT the "participant ID"
	ParticipantContextID string
	// DID the Decentralized Identifier of the participant, in most cases this is a "did:web"
	DID string
	// KeyGeneratorParameters the parameters used to generate the cryptographic key for the participant. Supplying a pre-generated key is not supported yet.
	KeyGeneratorParameters KeyGeneratorParameters
	// VaultConfig configuration for accessing a vault
	VaultConfig edcv.VaultConfig
	// VaultCredentials credentials which are needed to get a JWT which is used to access a vault
	VaultCredentials edcv.VaultCredentials
}

type CreateParticipantContextResponse struct {
	STSClientID     string `json:"clientId"`
	STSClientSecret string `json:"clientSecret"`
}
type ParticipantManifestOptions func(*ParticipantManifest)

func NewParticipantManifest(
	participantContextID string,
	did string,
	credentialServiceURL string,
	protocolServiceURL string,
	opts ...ParticipantManifestOptions,
) ParticipantManifest {
	manifest := ParticipantManifest{
		ParticipantContextID: participantContextID,
		DID:                  did,
		CredentialServiceURL: credentialServiceURL,
		CredentialServiceID:  participantContextID + "-credentialservice",
		ProtocolServiceURL:   protocolServiceURL,
		ProtocolServiceID:    participantContextID + "-dsp",
		IsActive:             true,
		KeyGeneratorParameters: KeyGeneratorParameters{
			KeyID:           DefaultKeyID,
			PrivateKeyAlias: DefaultKeyID,
			KeyAlgorithm:    DefaultAlgorithm,
			Curve:           DefaultCurve,
		},
		VaultConfig: edcv.VaultConfig{
			SecretPath: "v1/participants",
			FolderPath: participantContextID + "/identityhub",
		},
	}
	for _, opt := range opts {
		opt(&manifest)
	}

	sanitizedKeyID := manifest.KeyGeneratorParameters.KeyID
	if !strings.HasPrefix(sanitizedKeyID, did) {
		if !strings.HasPrefix(sanitizedKeyID, "#") {
			sanitizedKeyID = "#" + sanitizedKeyID
		}
		sanitizedKeyID = did + sanitizedKeyID
		manifest.KeyGeneratorParameters.KeyID = sanitizedKeyID
		manifest.KeyGeneratorParameters.PrivateKeyAlias = sanitizedKeyID
	}

	return manifest
}

type CredentialType struct {
	Format                 string `json:"format" validate:"required"`
	Type                   string `json:"type" validate:"required"`
	CredentialDefinitionID string `json:"id" validate:"required"`
}

type CredentialRequest struct {
	IssuerDID   string           `json:"issuerDid" validate:"required"`
	HolderPID   string           `json:"holderPid" validate:"required"`
	Credentials []CredentialType `json:"credentials" validate:"required"`
}
