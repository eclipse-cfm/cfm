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

package launcher

import (
	"net/http"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/common/siglet"
	"github.com/eclipse-cfm/cfm/agent/orchestration/siglet/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/tokenexchange"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	ActivityType           = "siglet-activity"
	sigletManagementURLKey = "siglet.management.url"
	sigletSignalingURLKey  = "siglet.signaling.url"
	controlPlaneURLKey     = "controlplane.url"
	tokenExchangeURLKey    = "tokenexchange.url"
	tokenFilePathKey       = "tokenexchange.tokenFilePath"
	audienceKey            = "tokenexchange.audience"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "Siglet Agent",
		ServiceName:  "cfm.agent.siglet",
		ConfigPrefix: "sigletagent",
		ActivityType: ActivityType,
		AssemblyProvider: func() []system.ServiceAssembly {
			return []system.ServiceAssembly{
				&httpclient.HttpClientServiceAssembly{},
			}
		},
		NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
			httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)
			sigletManagementURL := ctx.Config.GetString(sigletManagementURLKey)
			sigletSignalingURL := ctx.Config.GetString(sigletSignalingURLKey)
			cpURL := ctx.Config.GetString(controlPlaneURLKey)

			if err := runtime.CheckRequiredParams(
				sigletManagementURLKey, sigletManagementURL,
				sigletSignalingURLKey, sigletSignalingURL,
				controlPlaneURLKey, cpURL,
			); err != nil {
				panic(err)
			}

			provider := tokenexchange.NewTokenExchangeProvider(ctx.Config.GetString(tokenFilePathKey),
				tokenexchange.WithTokenExchangeUrl(ctx.Config.GetString(tokenExchangeURLKey)),
				tokenexchange.WithTokenExchangeAudience(ctx.Config.GetString(audienceKey)),
				tokenexchange.WithHttpClient(&httpClient))

			return activity.NewProcessor(&activity.Config{
				LogMonitor:                ctx.Monitor,
				SigletSignalingURL:        sigletSignalingURL,
				TransferTypeMappingClient: siglet.NewTransferTypeMappingClient(&httpClient, provider, sigletManagementURL),
				DataPlaneClient: controlplane.HttpManagementAPIClient{
					BaseURL:       cpURL,
					TokenProvider: provider,
					HttpClient:    &httpClient,
				},
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
