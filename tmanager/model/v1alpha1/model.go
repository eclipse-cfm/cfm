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
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
)

// Request DTOs declare their constraints with `validate` tags, which ReadPayload enforces and the OpenAPI generator turns
// into required properties. Response-only DTOs use `required:"true"`, which only documents the response.
//
// Request DTOs accept at most 100 entries in each map and list, map keys of at most 256 characters and strings of at
// most 256 characters (512 for identifiers and issuers), so that a single request can't store arbitrarily large blobs.
// Non-empty list elements are checked with min=1, because the OpenAPI generator reads any "required" in the tag as the
// list itself being required.

// Entity holds the server-assigned identity of a returned entity.
type Entity struct {
	ID      string `json:"id" required:"true"`
	Version int64  `json:"version" required:"true"`
}

type NewTenant struct {
	Properties map[string]any `json:"properties,omitempty" validate:"max=100,dive,keys,max=256,endkeys"`
}

type Tenant struct {
	Entity
	NewTenant
}

type NewCell struct {
	ExternalID     string         `json:"externalId" validate:"max=256"`
	State          string         `json:"state" validate:"required"`
	StateTimestamp time.Time      `json:"stateTimestamp"`
	Properties     map[string]any `json:"properties,omitempty" validate:"max=100,dive,keys,max=256,endkeys"`
}

type Cell struct {
	Entity
	NewCell
}

type NewDataspaceProfile struct {
	DataspaceSpec DataspaceSpec  `json:"dataspaceSpec,omitempty"`
	Artifacts     []string       `json:"artifacts,omitempty" validate:"max=100,dive,min=1,max=256"`
	Properties    map[string]any `json:"properties,omitempty" validate:"max=100,dive,keys,max=256,endkeys"`
}

type NewDataspaceProfileDeployment struct {
	ProfileID string `json:"profileId" validate:"required,max=256"`
	CellID    string `json:"cellId" validate:"required,max=256"`
}

type DataspaceDeployment struct {
	DeployableEntity
	CellID         string         `json:"cellId,omitempty"`
	ExternalCellID string         `json:"externalCellId"`
	Properties     map[string]any `json:"properties,omitempty"`
}
type DataspaceProfile struct {
	Entity
	DataspaceSpec DataspaceSpec         `json:"dataspaceSpec,omitempty"`
	Artifacts     []string              `json:"artifacts,omitempty"`
	Deployments   []DataspaceDeployment `json:"deployments,omitempty"`
	Properties    map[string]any        `json:"properties,omitempty"`
}

type DataspaceSpec struct {
	ProtocolStack   []string         `json:"protocolStack,omitempty" validate:"max=100,dive,min=1,max=256"`
	CredentialSpecs []CredentialSpec `json:"credentialSpecs,omitempty" validate:"max=100,dive"`
}

type CredentialSpec struct {
	Id              string `json:"id" validate:"required,max=256"`
	Type            string `json:"type" validate:"required,max=256"`
	Issuer          string `json:"issuer" validate:"required,max=512"`
	Format          string `json:"format" validate:"required,max=256"`
	ParticipantRole string `json:"role,omitempty" validate:"max=256"`
}

type NewParticipantProfileDeployment struct {
	Identifier string `json:"identifier" validate:"required,max=512"`
	// CellID is not used yet: the cell is chosen by the cell selector
	CellID              string                    `json:"cellId,omitempty" validate:"max=256"`
	DataspaceProfileIDs []string                  `json:"dataspaceProfileIds,omitempty" validate:"max=100,dive,min=1,max=256"`
	ParticipantRoles    map[string][]string       `json:"participantRoles,omitempty" validate:"max=100,dive,keys,max=256,endkeys,max=100,dive,min=1,max=256"`
	VPAProperties       map[string]map[string]any `json:"vpaProperties,omitempty" validate:"max=100,dive,keys,max=256,endkeys,max=100,dive,keys,max=256,endkeys"`
	Properties          map[string]any            `json:"properties,omitempty" validate:"max=100,dive,keys,max=256,endkeys"`
}

type ParticipantProfile struct {
	Entity
	Identifier       string                    `json:"identifier" required:"true"`
	TenantID         string                    `json:"tenantId"`
	ParticipantRoles map[string][]string       `json:"participantRoles"`
	VPAs             []VirtualParticipantAgent `json:"vpas,omitempty"`
	Properties       map[string]any            `json:"properties,omitempty"`
	Error            bool                      `json:"error"`
	ErrorDetail      string                    `json:"errorDetail,omitempty"`
	// ErrorCause is the underlying error. It is only returned to callers with the admin scope.
	ErrorCause string `json:"errorCause,omitempty"`
}

type VirtualParticipantAgent struct {
	DeployableEntity
	Type       model.VPAType  `json:"type" required:"true"`
	CellID     string         `json:"cellId" required:"true"`
	Properties map[string]any `json:"properties,omitempty"`
}

type DeployableEntity struct {
	Entity
	State          string    `json:"state" required:"true"`
	StateTimestamp time.Time `json:"stateTimestamp" required:"true"`
}

type TenantPropertiesDiff struct {
	Properties map[string]any `json:"properties" validate:"max=100,dive,keys,max=256,endkeys"`
	Removed    []string       `json:"removed" validate:"max=100,dive,max=256"`
}
