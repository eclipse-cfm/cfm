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

// Package dataspace defines the dataspace model that the Tenant Manager and the dataspace agents share: the virtual
// participant agent (VPA) types, the VPA orchestration types, the keys of the VPA orchestration payload, and the
// validation of participant context IDs.
//
// The Provision Manager is a generic orchestration engine and must not import this package.
package dataspace

import (
	"fmt"
	"regexp"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/go-playground/validator/v10"
)

const (
	ConnectorType         VPAType = "cfm.connector"
	CredentialServiceType VPAType = "cfm.credentialservice"
	DataPlaneType         VPAType = "cfm.dataplane"
	IssuerServiceType     VPAType = "cfm.issuer"
	ParticipantIdentifier         = "cfm.participant.id"

	VPADeployType  model.OrchestrationType = "cfm.orchestration.vpa.deploy"
	VPADisposeType model.OrchestrationType = "cfm.orchestration.vpa.dispose"

	VPAData        = "cfm.vpa.data"
	CredentialData = "cfm.vpa.credentials"
	VPAStateData   = "cfm.vpa.state"
)

// ParticipantContextIDTag is the validation tag that checks a field with ValidateParticipantContextID. The package
// registers it on model.Validator when it is initialized.
const ParticipantContextIDTag = "participantcontextid"

func init() {
	_ = model.Validator.RegisterValidation(ParticipantContextIDTag, func(fl validator.FieldLevel) bool {
		return ValidateParticipantContextID(fl.Field().String()) == nil
	})
}

type VPAType string

func (dt VPAType) String() string {
	return string(dt)
}

// participantContextIDPattern restricts participant context IDs to characters that are safe to use verbatim as a URL path
// segment and as a Vault folder name. Generated IDs are 32 hex characters, so the length cap leaves ample headroom.
var participantContextIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// ValidateParticipantContextID returns an error if the given ID contains anything other than letters, digits, underscores
// and hyphens, or is empty or longer than 128 characters. This rejects path separators, dots, query and fragment
// delimiters and percent-encoding, which could otherwise re-target admin API requests or Vault paths.
func ValidateParticipantContextID(id string) error {
	if !participantContextIDPattern.MatchString(id) {
		return fmt.Errorf("invalid participant context ID %q", id)
	}
	return nil
}

// VPAManifest represents the configuration details for a VPA deployment.
type VPAManifest struct {
	ID             string         `json:"id" validate:"required"`
	VPAType        VPAType        `json:"vpaType" validate:"required"`
	CellID         string         `json:"cellId" validate:"required"`
	ExternalCellID string         `json:"externalCellId"`
	Properties     map[string]any `json:"properties,omitempty"`
}

type CredentialSpec struct {
	Id              string `json:"id" validate:"required"`
	Type            string `json:"type" validate:"required"`
	Issuer          string `json:"issuer" validate:"required"`
	Format          string `json:"format" validate:"required"`
	ParticipantRole string `json:"role"`
}
