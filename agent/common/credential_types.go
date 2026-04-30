/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
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

package common

import "time"

// VerifiableCredentialResource represents a container for verifiable credentials as returned by the Identity Hub API.
// This object contains the raw, signed verifiable credential data and additional metadata and state information, representing

type VerifiableCredentialResource struct {
	ParticipantContextID string              `json:"participantContextId" validate:"required"`
	IssuerID             string              `json:"issuerId" validate:"required"`
	HolderID             string              `json:"holderId" validate:"required"`
	Metadata             map[string]any      `json:"metadata" validate:"required"`
	State                int32               `json:"state" validate:"required"`
	VerifiableCredential CredentialContainer `json:"verifiableCredential" validate:"required"`
}

// VerifiableCredential models the EDC VerifiableCredential type.
type VerifiableCredential struct {
	ID                string              `json:"id,omitempty" validate:"required"`
	Types             []string            `json:"type" validate:"required"` // at least one required
	Issuer            Issuer              `json:"issuer" validate:"required"`
	IssuanceDate      time.Time           `json:"issuanceDate" validate:"required"` // validFrom alias
	ExpirationDate    *time.Time          `json:"expirationDate,omitempty"`         // validUntil alias
	CredentialStatus  []CredentialStatus  `json:"credentialStatus,omitempty"`
	CredentialSchema  []CredentialSchema  `json:"credentialSchema,omitempty"`
	CredentialSubject []CredentialSubject `json:"credentialSubject"`
	Name              string              `json:"name,omitempty"`
	Description       string              `json:"description,omitempty"`
	DataModelVersion  DataModelVersion    `json:"dataModelVersion,omitempty"`
}

type CredentialFormat string

const (
	CredentialFormat_VCDM10_LD     CredentialFormat = "VC1_0_LD"
	CredentialFormat_VCDM10_JWT    CredentialFormat = "VC1_0_JWT"
	CredentialFormat_VCDM20_JOSE   CredentialFormat = "VC2_0_JOSE"
	CredentialFormat_VCDM20_SD_JWT CredentialFormat = "VC2_0_SD_JWT"
	CredentialFormat_VCDM20_COSE   CredentialFormat = "VC2_0_COSE"
)

// Issuer can be either a simple URI or an object with an ID.
// We'll model it as a struct with a flexible form.
type Issuer struct {
	ID string `json:"id"`
	// consider adding additional fields if full object support is needed
}

// CredentialStatus represents credentialStatus entries.
type CredentialStatus struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`
}

// CredentialSchema represents credentialSchema entries.
type CredentialSchema struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`
}

// CredentialSubject is a generic subject type (claim map could be used too).
type CredentialSubject = map[string]interface{}

// DataModelVersion mirrors the Java enum (V_1_1, etc.).
type DataModelVersion string

const (
	DataModelV1_1 DataModelVersion = "V_1_1"
	DataModelV2_0 DataModelVersion = "V_2_0"
	// add other versions if needed
)

type CredentialContainer struct {
	RawCredential    string               `json:"rawVc" validate:"required"`
	CredentialFormat CredentialFormat     `json:"format" validate:"required"`
	Credential       VerifiableCredential `json:"credential" validate:"required"`
}
