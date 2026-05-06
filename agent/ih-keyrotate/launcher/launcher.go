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
	"github.com/eclipse-cfm/cfm/agent/ih-keyrotate/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/oauth2"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	ActivityType      = "ih-keyrotation-activity"
	identityHubURLKey = "identityhub.url"
	clientIDKey       = "keycloak.clientID"
	clientSecretKey   = "keycloak.clientSecret"
	tokenUrlKey       = "keycloak.tokenUrl"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {

	config := natsagent.LauncherConfig{
		AgentName:    "IdentityHub Key Rotation Agent",
		ServiceName:  "cfm.agent.identityhub.keyrotate",
		ConfigPrefix: "ih-keyrotate",
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
			tokenUrl := ctx.Config.GetString(tokenUrlKey)
			if err := runtime.CheckRequiredParams(identityHubURLKey, ihURL, clientIDKey, clientID, clientSecretKey, clientSecret, tokenUrlKey, tokenUrl); err != nil {
				panic(err)
			}

			provider := oauth2.NewTokenProvider(
				oauth2.Oauth2Params{
					ClientID:     clientID,
					ClientSecret: clientSecret,
					TokenURL:     tokenUrl,
					GrantType:    oauth2.ClientCredentials,
				}, &httpClient)

			return activity.NewProcessor(identityhub.HttpIdentityAPIClient{
				BaseURL:       ihURL,
				TokenProvider: provider,
				HttpClient:    &httpClient,
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
