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
	"github.com/eclipse-cfm/cfm/agent/ih/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/assembly/vault"
	"github.com/eclipse-cfm/cfm/common/oauth2"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	ActivityType                       = "identityhub-activity"
	urlKey                             = "vault.url"
	clientIDKey                        = "keycloak.clientID"
	clientSecretKey                    = "keycloak.clientSecret"
	tokenURLKey                        = "keycloak.tokenUrl"
	identityHubURLKey                  = "identityhub.url"
	identityHubCredentialServiceURLKey = "identityhub.cs.url"
	controlPlaneProtocolURLKey         = "controlplane.protocol.url"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "IdentityHub Agent",
		ServiceName:  "cfm.agent.identityhub",
		ConfigPrefix: "ihagent",
		ActivityType: ActivityType,
		AssemblyProvider: func() []system.ServiceAssembly {
			return []system.ServiceAssembly{
				&httpclient.HttpClientServiceAssembly{},
				&vault.VaultServiceAssembly{},
			}
		},
		NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
			httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)
			vaultClient := ctx.Registry.Resolve(serviceapi.VaultKey).(serviceapi.VaultClient)
			clientID := ctx.Config.GetString(clientIDKey)
			clientSecret := ctx.Config.GetString(clientSecretKey)
			tokenURL := ctx.Config.GetString(tokenURLKey)
			ihURL := ctx.Config.GetString(identityHubURLKey)
			vaultURL := ctx.Config.GetString(urlKey)
			ihCsURL := ctx.Config.GetString(identityHubCredentialServiceURLKey)
			cpProtocolURL := ctx.Config.GetString(controlPlaneProtocolURLKey)

			if err := runtime.CheckRequiredParams(clientIDKey, clientID, clientSecretKey, clientSecret, tokenURLKey, tokenURL, identityHubURLKey, ihURL); err != nil {
				panic(err)
			}

			provider := oauth2.NewTokenProvider(
				oauth2.Oauth2Params{
					ClientID:     clientID,
					ClientSecret: clientSecret,
					TokenURL:     tokenURL,
					GrantType:    oauth2.ClientCredentials,
				}, &httpClient)

			return activity.NewProcessor(&activity.Config{
				VaultClient: vaultClient,
				Client:      &httpClient,
				LogMonitor:  ctx.Monitor,
				IdentityAPIClient: identityhub.HttpIdentityAPIClient{
					BaseURL:       ihURL,
					TokenProvider: provider,
					HttpClient:    &httpClient,
				},
				TokenURL:             tokenURL,
				VaultURL:             vaultURL,
				CredentialServiceURL: ihCsURL,
				ProtocolServiceURL:   cpProtocolURL,
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
