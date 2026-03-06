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
	"github.com/eclipse-cfm/cfm/agent/edcv/activity"
	"github.com/eclipse-cfm/cfm/agent/edcv/controlplane"
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
	urlKey                             = "vault.url" // duplicate of common/vault/assembly.go
	ActivityType                       = "edcv-activity"
	clientIDKey                        = "keycloak.clientID"
	clientSecretKey                    = "keycloak.clientSecret"
	tokenURLKey                        = "keycloak.tokenUrl"
	identityHubURLKey                  = "identityhub.url"
	identityHubStsURLKey               = "identityhub.sts.url"
	identityHubCredentialServiceURLKey = "identityhub.cs.url"
	controlPlaneURLKey                 = "controlplane.url"
	controlPlaneProtocolURLKey         = "controlplane.protocol.url"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "EDC-V Agent",
		ConfigPrefix: "edcvagent",
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
			tokenURL := ctx.Config.GetString(tokenURLKey) // this may be nil or "" if the in-mem vault is used
			ihURL := ctx.Config.GetString(identityHubURLKey)
			ihStsURL := ctx.Config.GetString(identityHubStsURLKey)
			cpURL := ctx.Config.GetString(controlPlaneURLKey)
			vaultURL := ctx.Config.GetString(urlKey) // this may be nil or "" if the in-mem vault is used
			cpProtocolURL := ctx.Config.GetString(controlPlaneProtocolURLKey)
			ihCsURL := ctx.Config.GetString(identityHubCredentialServiceURLKey)

			if err := runtime.CheckRequiredParams(clientIDKey, clientID, clientSecretKey, clientSecret, identityHubURLKey, ihURL, controlPlaneURLKey, cpURL, tokenURLKey, tokenURL, identityHubStsURLKey, ihStsURL); err != nil {
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
				STSTokenURL:          ihStsURL,
				CredentialServiceURL: ihCsURL,
				ProtocolServiceURL:   cpProtocolURL,
				ManagementAPIClient: controlplane.HttpManagementAPIClient{
					BaseURL:       cpURL,
					TokenProvider: provider,
					HttpClient:    &httpClient,
				},
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
