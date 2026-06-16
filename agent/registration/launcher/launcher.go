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

package launcher

import (
	"net/http"

	"github.com/eclipse-cfm/cfm/agent/common/issuerservice"
	"github.com/eclipse-cfm/cfm/agent/registration/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/tokenexchange"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	ActivityType            = "registration-activity"
	issuerServiceBaseUrlKey = "issuerservice.url"
	issuerIDKey             = "issuer.id"
	tokenExchangeURLKey     = "tokenexchange.url"
	tokenFilePathKey        = "tokenexchange.tokenFilePath"
	audienceKey             = "tokenexchange.audience"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "Registration Agent",
		ServiceName:  "cfm.agent.registration",
		ConfigPrefix: "regagent",
		ActivityType: ActivityType,
		AssemblyProvider: func() []system.ServiceAssembly {
			return []system.ServiceAssembly{
				&httpclient.HttpClientServiceAssembly{},
			}
		},
		NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
			httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)
			issuerServiceBaseUrl := ctx.Config.GetString(issuerServiceBaseUrlKey)
			issuerID := ctx.Config.GetString(issuerIDKey)

			if err := runtime.CheckRequiredParams(issuerServiceBaseUrlKey, issuerServiceBaseUrl, issuerIDKey, issuerID); err != nil {
				panic(err)
			}

			provider := tokenexchange.NewTokenExchangeProvider(ctx.Config.GetString(tokenFilePathKey),
				tokenexchange.WithTokenExchangeUrl(ctx.Config.GetString(tokenExchangeURLKey)),
				tokenexchange.WithTokenExchangeAudience(ctx.Config.GetString(audienceKey)),
				tokenexchange.WithHttpClient(&httpClient))
			return activity.NewProcessor(&activity.Config{
				LogMonitor: ctx.Monitor,
				IssuerService: issuerservice.HttpApiClient{
					BaseURL:       issuerServiceBaseUrl,
					TokenProvider: provider,
					IssuerID:      issuerID,
					HttpClient:    &httpClient,
				},
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
