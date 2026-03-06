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

package natsprovision

import (
	"context"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

// provisionCallbackDispatcher routes provision responses to the associated handler.
type provisionCallbackDispatcher interface {

	// Dispatch is invoked when an orchestration is complete.
	// If a recoverable error is encountered one of model.RecoverableError, model.ClientError, or model.FatalError will be returned.
	Dispatch(ctx context.Context, response model.OrchestrationResponse) error
}

// provisionCallbackService registers api.ProvisionCallbackHandler instances and dispatches orchestration responses.
type provisionCallbackService struct {
	handlers map[string]api.ProvisionCallbackHandler
}

func newProvisionCallbackService() *provisionCallbackService {
	return &provisionCallbackService{handlers: make(map[string]api.ProvisionCallbackHandler)}
}
func (d provisionCallbackService) Register(orchestrationType model.OrchestrationType, handler api.ProvisionCallbackHandler) {
	d.handlers[orchestrationType.String()] = handler
}

func (d provisionCallbackService) Dispatch(ctx context.Context, response model.OrchestrationResponse) error {
	handler, found := d.handlers[response.OrchestrationType.String()]
	if !found {
		return types.NewFatalError("provision handler not found for type: %s", response.OrchestrationType)
	}
	return handler(ctx, response)
}
