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

package launcher

import (
	"net/http"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/common/siglet"
	"github.com/eclipse-cfm/cfm/agent/lifecycle/keymanagementagent/handler"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/tokenexchange"
)

const (
	// KeyPairSubject is the wildcard subject the agent subscribes to; it covers all key management
	// lifecycle events (created, updated, ...).
	KeyPairSubject               = "events.keypair.>"
	tokenExchangeURLKey          = "tokenexchange.url"
	tokenFilePathKey             = "tokenexchange.tokenFilePath"
	audienceKey                  = "tokenexchange.audience"
	sigletManagementUrlKey       = "siglet.management.url"
	controlPlaneManagementURLKey = "controlplane.management.url"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {

	config := lifecycleagent.LauncherConfig[handler.KeyManagementEvent]{
		AgentName:    "Key Management Agent",
		ServiceName:  "cfm.agent.keymanagement",
		ConfigPrefix: "kmagent",
		Subjects:     []string{KeyPairSubject},
		AssemblyProvider: func() []system.ServiceAssembly {
			return []system.ServiceAssembly{
				&httpclient.HttpClientServiceAssembly{},
			}
		},
		NewProcessor: func(ctx *lifecycleagent.AgentContext) lifecycleagent.EventProcessor[handler.KeyManagementEvent] {
			httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)
			sigletManagementAPIUrl := ctx.Config.GetString(sigletManagementUrlKey)
			controlPlaneURL := ctx.Config.GetString(controlPlaneManagementURLKey)

			if err := runtime.CheckRequiredParams(
				sigletManagementUrlKey, sigletManagementAPIUrl,
				controlPlaneManagementURLKey, controlPlaneURL,
			); err != nil {
				panic(err)
			}

			provider := tokenexchange.NewTokenExchangeProvider(ctx.Config.GetString(tokenFilePathKey),
				tokenexchange.WithTokenExchangeUrl(ctx.Config.GetString(tokenExchangeURLKey)),
				tokenexchange.WithTokenExchangeAudience(ctx.Config.GetString(audienceKey)),
				tokenexchange.WithHttpClient(&httpClient))

			return handler.NewProcessor(&handler.Config{
				LogMonitor:   ctx.Monitor,
				SigletClient: siglet.NewSigletAPIClient(&httpClient, provider, sigletManagementAPIUrl),
				ControlPlaneClient: controlplane.HttpManagementAPIClient{
					BaseURL:       controlPlaneURL,
					TokenProvider: provider,
					HttpClient:    &httpClient,
				},
			})
		},
	}
	lifecycleagent.LaunchAgent(shutdown, config)
}
