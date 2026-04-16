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
	"context"
	"fmt"
	"net/http"

	"github.com/eclipse-cfm/cfm/agent/keycloak/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/assembly/vault"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
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
		ServiceName:  "cfm.agent.keycloak",
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
	_, err := initTracer()
	if err != nil {
		panic(err)
	}
	natsagent.LaunchAgent(shutdown, config)
}

func initTracer() (*trace.TracerProvider, error) {
	ctx := context.Background()
	spanExporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(semconv.ServiceNameKey.String(AgentPrefix)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry resource: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(spanExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}
