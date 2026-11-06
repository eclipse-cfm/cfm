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

// Entity holds the server-assigned identity of a returned entity.
type Entity struct {
	ID      string `json:"id" required:"true"`
	Version int64  `json:"version" required:"true"`
}

type NewTenant struct {
	Properties map[string]any `json:"properties,omitempty"`
}

type Tenant struct {
	Entity
	NewTenant
}

type NewCell struct {
	ExternalID     string         `json:"externalId"`
	State          string         `json:"state" validate:"required"`
	StateTimestamp time.Time      `json:"stateTimestamp"`
	Properties     map[string]any `json:"properties,omitempty"`
}

type Cell struct {
	Entity
	NewCell
}

type NewDataspaceProfile struct {
	DataspaceSpec DataspaceSpec  `json:"dataspaceSpec,omitempty"`
	Artifacts     []string       `json:"artifacts,omitempty"`
	Properties    map[string]any `json:"properties,omitempty"`
}

type NewDataspaceProfileDeployment struct {
	ProfileID string `json:"profileId" validate:"required"`
	CellID    string `json:"cellId" validate:"required"`
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
	ProtocolStack   []string         `json:"protocolStack,omitempty"`
	CredentialSpecs []CredentialSpec `json:"credentialSpecs,omitempty" validate:"dive"`
}

type CredentialSpec struct {
	Id              string `json:"id" validate:"required"`
	Type            string `json:"type" validate:"required"`
	Issuer          string `json:"issuer" validate:"required"`
	Format          string `json:"format" validate:"required"`
	ParticipantRole string `json:"role,omitempty"`
}

type NewParticipantProfileDeployment struct {
	Identifier string `json:"identifier" validate:"required"`
	// CellID is not used yet: the cell is chosen by the cell selector
	CellID              string                    `json:"cellId,omitempty"`
	DataspaceProfileIDs []string                  `json:"dataspaceProfileIds,omitempty"`
	ParticipantRoles    map[string][]string       `json:"participantRoles,omitempty"`
	VPAProperties       map[string]map[string]any `json:"vpaProperties,omitempty"`
	Properties          map[string]any            `json:"properties,omitempty"`
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
	Properties map[string]any `json:"properties"`
	Removed    []string       `json:"removed"`
}
