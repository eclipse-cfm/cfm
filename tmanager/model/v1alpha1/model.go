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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
)

var iso8601DurationRe = regexp.MustCompile(`^P(?:\d+Y)?(?:\d+M)?(?:\d+W)?(?:\d+D)?(?:T(?:\d+H)?(?:\d+M)?(?:\d+S)?)?$`)

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
	State          string         `json:"state" required:"true"`
	StateTimestamp time.Time      `json:"stateTimestamp" required:"true"`
	Properties     map[string]any `json:"properties,omitempty"`
}

type Cell struct {
	Entity
	NewCell
}

type NewDataspaceProfile struct {
	DataspaceSpec DataspaceSpec  `json:"dataspaceSpec,omitempty"`
	Artifacts     []string       ` json:"artifacts,omitempty"`
	Properties    map[string]any `json:"properties,omitempty"`
}

type NewDataspaceProfileDeployment struct {
	ProfileID string `json:"profileId" required:"true"`
	CellID    string `json:"cellId,omitempty"`
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
	CredentialSpecs []CredentialSpec `json:"credentialSpecs,omitempty"`
}

type CredentialSpec struct {
	Id              string `json:"id" required:"true"`
	Type            string `json:"type" required:"true"`
	Issuer          string `json:"issuer" required:"true"`
	Format          string `json:"format" required:"true"`
	ParticipantRole string `json:"role,omitempty"`
}

type NewParticipantProfileDeployment struct {
	Identifier          string                    `json:"identifier" required:"true"`
	CellID              string                    `json:"cellId" required:"true"`
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

// KeyRotationRequest represents a request to rotate a key, with optional parameters for algorithm (default: eddsa), curve (default: ed25519), and
// grace period (default: P3M, 3 months).
type KeyRotationRequest struct {
	KeyID       string          `json:"keyId" required:"true"`
	Algorithm   string          `json:"algorithm,omitempty"`
	Curve       string          `json:"curve,omitempty"`
	GracePeriod DurationISO8601 `json:"gracePeriod,omitempty"`
}

// UnmarshalJSON implements the json.Unmarshaler interface to provide default values
func (r *KeyRotationRequest) UnmarshalJSON(b []byte) error {
	r.Algorithm = "eddsa"
	r.Curve = "ed25519"
	r.GracePeriod = NewDuration("P3M")

	// use a type alias to break infinite recursion
	type Alias KeyRotationRequest
	return json.Unmarshal(b, (*Alias)(r))
}

// DurationISO8601 is an ISO 8601 duration (e.g. "P3M", "P1Y2M3DT4H5M6S").
type DurationISO8601 struct {
	raw string
}

// NewDuration creates a new DurationISO8601 from a string and panics if the string is not a valid ISO 8601 duration.
func NewDuration(s string) DurationISO8601 {
	if !iso8601DurationRe.MatchString(s) {
		panic(fmt.Errorf("invalid ISO 8601 duration: %q", s))
	}
	return DurationISO8601{raw: s}
}

func (d *DurationISO8601) String() string {
	return d.raw
}

func (d *DurationISO8601) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if !iso8601DurationRe.MatchString(s) {
		return fmt.Errorf("invalid ISO 8601 duration: %q", s)
	}
	d.raw = s
	return nil
}

func (d *DurationISO8601) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.raw + `"`), nil
}
