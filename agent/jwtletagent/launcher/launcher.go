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

	"github.com/eclipse-cfm/cfm/agent/jwtletagent/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/tokenexchange"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	ActivityType        = "jwtlet-activity"
	tokenExchangeURLKey = "tokenexchange.url"
	managementUrlKey    = "management.url"
	tokenFilePathKey    = "tokenexchange.tokenFilePath"
	audienceKey         = "tokenexchange.audience"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "Jwtlet Agent",
		ServiceName:  "cfm.agent.jwtlet",
		ConfigPrefix: "jwtletagent",
		ActivityType: ActivityType,
		AssemblyProvider: func() []system.ServiceAssembly {
			return []system.ServiceAssembly{
				&httpclient.HttpClientServiceAssembly{},
			}
		},
		NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
			httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)
			tokenExchangeURL := ctx.Config.GetString(tokenExchangeURLKey)
			tokenFilePath := ctx.Config.GetString(tokenFilePathKey)
			audience := ctx.Config.GetString(audienceKey)
			managementBasePath := ctx.Config.GetString(managementUrlKey)

			if err := runtime.CheckRequiredParams(tokenExchangeURLKey, tokenExchangeURL, tokenFilePathKey, tokenFilePath, managementUrlKey, managementBasePath); err != nil {
				panic(err)
			}

			provider := tokenexchange.NewTokenExchangeProvider(
				tokenFilePath,
				tokenexchange.WithTokenExchangeUrl(tokenExchangeURL),
				tokenexchange.WithTokenExchangeAudience(audience),
				tokenexchange.WithHttpClient(&httpClient),
			)

			return activity.NewProcessor(&activity.Config{
				LogMonitor:         ctx.Monitor,
				TokenProvider:      provider,
				HttpClient:         &httpClient,
				TokenFilePath:      tokenFilePath,
				Audience:           audience,
				ManagementBasePath: managementBasePath,
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
