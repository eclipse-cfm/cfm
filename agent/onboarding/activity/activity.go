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

package activity

import (
	"fmt"
	"time"

	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"github.com/eclipse-cfm/cfm/agent/common/issuerservice"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type OnboardingActivityProcessor struct {
	api.BaseActivityProcessor
	Monitor                system.LogMonitor
	IdentityApiClient      identityhub.IdentityAPIClient
	IssuerServiceApiClient issuerservice.ApiClient
	tracer                 trace.Tracer
}

type credentialRequestData struct {
	//CredentialRequest    identityhub.CredentialRequest `json:"credentialRequest"`
	ParticipantContextID string `json:"clientID.apiAccess" validate:"required"`
	HolderPID            string `json:"holderPid"`
	CredentialRequestURL string `json:"credentialRequest"`
}

// ProcessDeploy processes the deploy action for an onboarding activity by requesting the issuance of verifiable credentials.
// if a credential request is in progress, the deploy action simply checks its status and reschedules itself.
func (p OnboardingActivityProcessor) ProcessDeploy(ctx api.ActivityContext) api.ActivityResult {
	var credentialRequest credentialRequestData
	if err := ctx.ReadValues(&credentialRequest); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Onboarding activity for orchestration %s: %w", ctx.OID(), err)}
	}

	// no credential request was made yet -> make one
	if "" == credentialRequest.HolderPID {
		var data onboardingData
		if err := ctx.ReadValues(&data); err != nil {
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Onboarding activity for orchestration %s: %w", ctx.OID(), err)}
		}
		return p.processNewRequest(ctx, &data, credentialRequest)
	}
	// holderPID exists, check the status of the issuance
	return p.processExistingRequest(ctx, credentialRequest)
}

// ProcessDispose revokes a credential using the IssuerService's Admin API. The credential is NOT deleted from the
// holder wallet (IdentityHub).
func (p OnboardingActivityProcessor) ProcessDispose(ctx api.ActivityContext) api.ActivityResult {

	var credentialRequest credentialRequestData
	if err := ctx.ReadValues(&credentialRequest); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Onboarding activity for orchestration %s: %w", ctx.OID(), err)}
	}

	var data onboardingData
	if err := ctx.ReadValues(&data); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error processing Onboarding activity for orchestration %s: %w", ctx.OID(), err)}
	}

	participantContextID := credentialRequest.ParticipantContextID
	// query credentials by type, using the IdentityAPI
	var revocationErrors []error

	for _, spec := range data.CredentialSpecs {
		credentialType := spec.Type

		credentials, err := p.IdentityApiClient.QueryCredentialByType(participantContextID, credentialType)
		if err != nil {
			return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error querying credentials by type %s for participant context %s: %w", credentialType, participantContextID, err)}
		}

		if len(credentials) == 0 {
			p.Monitor.Infof("Rollback: could not revoke credentials of type '%s': 0 found for participant context '%s'", credentialType, participantContextID)
			return api.ActivityResult{Result: api.ActivityResultComplete}
		}

		// for each credential, send a revocation request
		for _, credential := range credentials {
			err := p.IssuerServiceApiClient.RevokeCredential(participantContextID, credential.VerifiableCredential.Credential.ID)
			if err != nil {
				revocationErrors = append(revocationErrors, err)
			}
		}
	}

	if len(revocationErrors) > 0 {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error revoking one or more credentials: %v", revocationErrors)}
	}

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p OnboardingActivityProcessor) processExistingRequest(ctx api.ActivityContext, credentialRequest credentialRequestData) api.ActivityResult {
	tracer := otel.GetTracerProvider().Tracer("cfm.agent.onboarding")
	_, span := tracer.Start(ctx.Context(), "cfm.agent.onboarding.deploy.check-credentials", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	state, err := p.IdentityApiClient.GetCredentialRequestState(credentialRequest.ParticipantContextID, credentialRequest.HolderPID)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error getting credential request state: %w", err)}
	}
	p.Monitor.Infof("Credential request for participant '%s' is in state '%s'", credentialRequest.ParticipantContextID, state)

	span.SetAttributes(attribute.String("state", state))

	switch state {

	case identityhub.CredentialRequestStateCreated:
		return api.ActivityResult{Result: api.ActivityResultSchedule, WaitOnReschedule: time.Duration(5) * time.Second}
	case identityhub.CredentialRequestStateIssued:
		ctx.SetOutputValue("holderPid", credentialRequest.HolderPID)
		ctx.SetOutputValue("credentialRequest", credentialRequest.CredentialRequestURL)
		ctx.SetOutputValue("participantContextId", credentialRequest.ParticipantContextID)
		return api.ActivityResult{Result: api.ActivityResultComplete}
	case identityhub.CredentialRequestStateRejected:
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("credential request for participant '%s' was rejected", credentialRequest.ParticipantContextID)}
	case identityhub.CredentialRequestStateError:
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("credential request for participant '%s' failed", credentialRequest.ParticipantContextID)}
	default:
		return api.ActivityResult{Result: api.ActivityResultRetryError, Error: fmt.Errorf("unexpected credential request state '%s'", state)}
	}
}

func (p OnboardingActivityProcessor) processNewRequest(ctx api.ActivityContext, data *onboardingData, credentialRequest credentialRequestData) api.ActivityResult {
	tracer := otel.GetTracerProvider().Tracer("cfm.agent.onboarding")
	_, span := tracer.Start(ctx.Context(), "cfm.agent.onboarding.request-credentials", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	if len(data.CredentialSpecs) == 0 {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("no credential specs provided")}
	}

	var credentials []identityhub.CredentialType

	issuers := make(map[string]struct{})
	issuer := ""
	var credentialsTypes []string
	for _, spec := range data.CredentialSpecs {
		issuer = spec.Issuer
		issuers[issuer] = struct{}{}
		credentials = append(credentials, identityhub.CredentialType{
			Format:                 spec.Format,
			Type:                   spec.Type,
			CredentialDefinitionID: spec.Id,
		})
		credentialsTypes = append(credentialsTypes, spec.Type)
	}

	if len(issuers) > 1 {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("multiple issuers not supported yet")}
	}

	holderPid := uuid.New().String()
	cr := identityhub.CredentialRequest{
		IssuerDID:   issuer,
		HolderPID:   holderPid,
		Credentials: credentials,
	}
	// make credential request
	location, err := p.IdentityApiClient.RequestCredentials(credentialRequest.ParticipantContextID, cr)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("error requesting credentials: %w", err)}
	}
	p.Monitor.Infof("Credentials request for participant '%s' and credentials '%s' submitted successfully, credential is at %s", credentialRequest.ParticipantContextID, credentialsTypes, location)
	ctx.SetValue("participantContextId", credentialRequest.ParticipantContextID)
	ctx.SetValue("holderPid", holderPid)
	ctx.SetValue("credentialRequest", location)

	span.SetAttributes(attribute.String("holderPid", holderPid), attribute.String("credentialRequest", location))

	return api.ActivityResult{
		Result:           api.ActivityResultSchedule,
		WaitOnReschedule: time.Duration(5) * time.Second,
		Error:            nil,
	}
}

type onboardingData struct {
	CredentialSpecs []model.CredentialSpec `json:"cfm.vpa.credentials" validate:"required,dive"`
}
