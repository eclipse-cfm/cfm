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

package model

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

const (
	ConnectorType         VPAType = "cfm.connector"
	CredentialServiceType VPAType = "cfm.credentialservice"
	DataPlaneType         VPAType = "cfm.dataplane"
	ParticipantIdentifier         = "cfm.participant.id"

	VPADeployType  OrchestrationType = "cfm.orchestration.vpa.deploy"
	VPADisposeType OrchestrationType = "cfm.orchestration.vpa.dispose"

	VPAData        = "cfm.vpa.data"
	CredentialData = "cfm.vpa.credentials"
	VPAStateData   = "cfm.vpa.state"
)

var Validator = initValidator()

// OrchestrationManifest represents the configuration details for the execution of an orchestration.
//
// The manifest includes a unique identifier, the orchestration type, and a payload of orchestration-specific data, which
// will be passed as input to the Orchestration.
type OrchestrationManifest struct {
	ID                string            `json:"id" validate:"required"`
	CorrelationID     string            `json:"correlationId" validate:"required"`
	OrchestrationType OrchestrationType `json:"orchestrationType" validate:"required"`
	Payload           map[string]any    `json:"payload,omitempty"`
}

// OrchestrationResponse returned when a system deployment completes.
type OrchestrationResponse struct {
	ID                string            `json:"id" validate:"required"`
	ManifestID        string            `json:"manifestId" validate:"required"`
	CorrelationID     string            `json:"correlationId" validate:"required"`
	OrchestrationType OrchestrationType `json:"orchestrationType" validate:"required"`
	Success           bool              `json:"success"`
	ErrorDetail       string            `json:"errorDetail,omitempty"`
	Properties        map[string]any    `json:"properties"`
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

type OrchestrationType string

func (dt OrchestrationType) String() string {
	return string(dt)
}

type VPAType string

func (dt VPAType) String() string {
	return string(dt)
}

type Query struct {
	Predicate string `json:"predicate"`
	Offset    int64  `json:"offset"`
	Limit     int64  `json:"limit"`
}

func initValidator() *validator.Validate {
	v := validator.New()

	_ = v.RegisterValidation("modeltype", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		// Only allow alphanumeric, dots, underscores, and hyphens
		match, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, value)
		return match
	})
	return v
}
