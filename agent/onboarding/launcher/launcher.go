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

	"github.com/eclipse-cfm/cfm/agent/common/identityhub"
	"github.com/eclipse-cfm/cfm/agent/common/issuerservice"
	"github.com/eclipse-cfm/cfm/agent/onboarding/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/oauth2"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	ActivityType            = "onboarding-activity"
	identityHubURLKey       = "identityhub.url"
	clientIDKey             = "keycloak.clientID"
	clientSecretKey         = "keycloak.clientSecret"
	tokenURLKey             = "keycloak.tokenUrl"
	issuerServiceBaseUrlKey = "issuerservice.url"
	issuerIDKey             = "issuer.id"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "Onboarding Agent",
		ServiceName:  "cfm.agent.onboarding",
		ConfigPrefix: "obagent",
		ActivityType: ActivityType,
		AssemblyProvider: func() []system.ServiceAssembly {
			return []system.ServiceAssembly{
				&httpclient.HttpClientServiceAssembly{},
			}
		},
		NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
			httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)
			ihURL := ctx.Config.GetString(identityHubURLKey)
			clientID := ctx.Config.GetString(clientIDKey)
			clientSecret := ctx.Config.GetString(clientSecretKey)
			issuerServiceBaseUrl := ctx.Config.GetString(issuerServiceBaseUrlKey)
			issuerID := ctx.Config.GetString(issuerIDKey)

			tokenURL := ctx.Config.GetString(tokenURLKey) // this may be nil or "" if the in-mem vault is used
			if err := runtime.CheckRequiredParams(identityHubURLKey, ihURL, clientIDKey, clientID, clientSecretKey, clientSecret, tokenURLKey, tokenURL); err != nil {
				panic(err)
			}

			provider := oauth2.NewTokenProvider(
				oauth2.Oauth2Params{
					ClientID:     clientID,
					ClientSecret: clientSecret,
					TokenURL:     tokenURL,
					GrantType:    oauth2.ClientCredentials,
				}, &httpClient)

			return activity.OnboardingActivityProcessor{
				Monitor: ctx.Monitor,
				IdentityApiClient: identityhub.HttpIdentityAPIClient{
					BaseURL:       ihURL,
					TokenProvider: provider,
					HttpClient:    &httpClient,
				},
				IssuerServiceApiClient: issuerservice.HttpApiClient{
					BaseURL:       issuerServiceBaseUrl,
					TokenProvider: provider,
					IssuerID:      issuerID,
					HttpClient:    &httpClient,
				},
			}
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
