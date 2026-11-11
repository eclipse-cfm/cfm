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
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

const (
	ConnectorType         VPAType = "cfm.connector"
	CredentialServiceType VPAType = "cfm.credentialservice"
	DataPlaneType         VPAType = "cfm.dataplane"
	IssuerServiceType     VPAType = "cfm.issuer"
	ParticipantIdentifier         = "cfm.participant.id"

	VPADeployType  OrchestrationType = "cfm.orchestration.vpa.deploy"
	VPADisposeType OrchestrationType = "cfm.orchestration.vpa.dispose"

	VPAData        = "cfm.vpa.data"
	CredentialData = "cfm.vpa.credentials"
	VPAStateData   = "cfm.vpa.state"
)

var Validator = initValidator()

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

// OrchestrationManifest represents the configuration details for the execution of an orchestration.
//
// The manifest includes a unique identifier, the orchestration type, and a payload of orchestration-specific data, which
// will be passed as input to the Orchestration.
type OrchestrationManifest struct {
	ID                string            `json:"id" validate:"required"`
	CorrelationID     string            `json:"correlationId" validate:"required"`
	OrchestrationType OrchestrationType `json:"orchestrationType" validate:"required"`
	Payload           map[string]any    `json:"payload,omitempty"`
	// CompensatedManifestID is set by the Provision Manager when it starts this orchestration to compensate a failed
	// one. It is never read from serialized manifests, so external callers cannot claim to compensate an orchestration.
	CompensatedManifestID string `json:"-"`
}

// OrchestrationResponse returned when a system deployment completes.
type OrchestrationResponse struct {
	ID                string            `json:"id" validate:"required"`
	ManifestID        string            `json:"manifestId" validate:"required"`
	CorrelationID     string            `json:"correlationId" validate:"required"`
	OrchestrationType OrchestrationType `json:"orchestrationType" validate:"required"`
	Success           bool              `json:"success"`
	// ErrorDetail says why the orchestration failed in words written for the caller that started it.
	ErrorDetail string `json:"errorDetail,omitempty"`
	// ErrorCause is the underlying error of a failed orchestration. It may quote internal addresses and downstream
	// responses, so it must only be shown to operators.
	ErrorCause string         `json:"errorCause,omitempty"`
	Properties map[string]any `json:"properties"`
	// CompensatedManifestID references the failed orchestration this orchestration compensated, if any.
	CompensatedManifestID string `json:"compensatedManifestId,omitempty"`
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
	Predicate string `json:"predicate" required:"true"`
	Offset    int64  `json:"offset"`
	Limit     int64  `json:"limit"`
}

func None() Query {
	return Query{
		Predicate: "true",
	}
}

func initValidator() *validator.Validate {
	v := validator.New()

	// Report fields by their JSON names, so that errors refer to what the client sent
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		switch name {
		case "-":
			return ""
		case "":
			return field.Name
		default:
			return name
		}
	})

	_ = v.RegisterValidation("modeltype", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		// Only allow alphanumeric, dots, underscores, and hyphens
		match, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, value)
		return match
	})

	// participantcontextid validates participant context IDs, see ValidateParticipantContextID
	_ = v.RegisterValidation("participantcontextid", func(fl validator.FieldLevel) bool {
		return ValidateParticipantContextID(fl.Field().String()) == nil
	})
	return v
}

// ValidationMessage describes a validation error returned by Validator as a list of the failed fields and checks, for
// example "field 'dataspaceSpec.credentialSpecs[0].id' failed the 'required' check". Other errors are returned as is.
func ValidationMessage(err error) string {
	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return err.Error()
	}
	messages := make([]string, 0, len(fieldErrors))
	for _, fieldError := range fieldErrors {
		// The namespace starts with the name of the validated struct, which means nothing to the client
		field := fieldError.Namespace()
		if _, rest, found := strings.Cut(field, "."); found {
			field = rest
		}
		check := fieldError.Tag()
		if fieldError.Param() != "" {
			check += "=" + fieldError.Param()
		}
		messages = append(messages, fmt.Sprintf("field '%s' failed the '%s' check", field, check))
	}
	return strings.Join(messages, "; ")
}
