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

	"github.com/eclipse-cfm/cfm/agent/keycloak/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/assembly/vault"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	ActivityType = "keycloak-activity"
	AgentPrefix  = "kcagent"
	urlKey       = "keycloak.url"
	realmKey     = "keycloak.realm"
	clientId     = "keycloak.clientid"
	username     = "keycloak.username"
	password     = "keycloak.password"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "KeyCloak Agent",
		ConfigPrefix: AgentPrefix,
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

			url := ctx.Config.GetString(urlKey)
			kcClientId := ctx.Config.GetString(clientId)
			kcUsername := ctx.Config.GetString(username)
			kcPassword := ctx.Config.GetString(password)
			realm := ctx.Config.GetString(realmKey)
			if err := runtime.CheckRequiredParams(urlKey, url, clientId, kcClientId, username, kcUsername, password, kcPassword, realmKey, realm); err != nil {
				panic(err)
			}
			return activity.NewProcessor(&activity.Config{
				KeycloakURL: url,
				ClientId:    kcClientId,
				Username:    kcUsername,
				Password:    kcPassword,
				Realm:       realm,
				VaultClient: vaultClient,
				HTTPClient:  &httpClient,
				Monitor:     ctx.Monitor,
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
