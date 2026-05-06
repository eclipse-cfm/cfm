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

	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	tmanager "github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/google/uuid"
)

// keyRotationData contains all relevant data for the key rotation activity and is read off of the ActivityContext
type keyRotationData struct {
	ParticipantIdentifier string                      `json:"cfm.participant.id" validate:"required"`
	RotationParams        tmanager.KeyRotationRequest `json:"cfm.key.rotation.data" validate:"required"`
	ParticipantContextID  string                      `json:"participantContextID" validate:"required"`
}
type KeyRotationActivityProcessor struct {
	api.BaseActivityProcessor
	IdentityAPIClient identityhub.IdentityAPIClient
}

type Config struct {
	IdentityHubURL string
}

func NewProcessor(ihApiClient identityhub.IdentityAPIClient) *KeyRotationActivityProcessor {
	return &KeyRotationActivityProcessor{
		IdentityAPIClient: ihApiClient,
	}
}

func (p KeyRotationActivityProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	var params keyRotationData
	err := ctx.ReadValues(&params)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}

	// call the keyrotation API
	keyId := params.ParticipantIdentifier + "#" + uuid.NewString()
	err = p.IdentityAPIClient.RotateKey(ctx.Context(), params.ParticipantContextID, keyId, params.RotationParams)
	if err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}

	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p KeyRotationActivityProcessor) ProcessDeploy(_ api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot use this agent on deploy")}
}

func (p KeyRotationActivityProcessor) ProcessDispose(_ api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("cannot use this agent on dispose")}
}
