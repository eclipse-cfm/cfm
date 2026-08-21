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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/eclipse-cfm/cfm/agent/orchestration/jwtletagent/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/tokenexchange"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
	"github.com/spf13/viper"
)

const (
	ActivityType        = "jwtlet-activity"
	tokenExchangeURLKey = "tokenexchange.url"
	managementUrlKey    = "management.url"
	tokenFilePathKey    = "tokenexchange.tokenFilePath"
	audienceKey         = "tokenexchange.audience"
	namespaceKey        = "serviceaccount.namespace"
	defaultNamespace    = "edc-v"
	// clientMappingsKey optionally overrides the workload ServiceAccounts that get a
	// participant-scoped resource mapping. When unset, the activity falls back to its built-in
	// defaults. See parseClientMappings for the accepted forms.
	clientMappingsKey = "clientmappings"
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
			ctx.Config.SetDefault(namespaceKey, defaultNamespace)
			tokenExchangeURL := ctx.Config.GetString(tokenExchangeURLKey)
			tokenFilePath := ctx.Config.GetString(tokenFilePathKey)
			audience := ctx.Config.GetString(audienceKey)
			managementBasePath := ctx.Config.GetString(managementUrlKey)
			namespace := ctx.Config.GetString(namespaceKey)

			if err := runtime.CheckRequiredParams(tokenExchangeURLKey, tokenExchangeURL, tokenFilePathKey, tokenFilePath, managementUrlKey, managementBasePath); err != nil {
				panic(err)
			}

			clientMappings, err := parseClientMappings(ctx.Config)
			if err != nil {
				panic(err)
			}

			provider := tokenexchange.NewTokenExchangeProvider(
				tokenFilePath,
				tokenexchange.WithTokenExchangeUrl(tokenExchangeURL),
				tokenexchange.WithTokenExchangeAudience(audience),
				tokenexchange.WithHttpClient(&httpClient),
			)

			return activity.NewProcessor(&activity.Config{
				LogMonitor:              ctx.Monitor,
				TokenProvider:           provider,
				HttpClient:              &httpClient,
				TokenFilePath:           tokenFilePath,
				Audience:                audience,
				ManagementBasePath:      managementBasePath,
				ServiceAccountNamespace: namespace,
				ClientMappings:          clientMappings,
			})
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}

// parseClientMappings reads the optional `clientmappings` key, which overrides the workload
// ServiceAccounts that get a participant-scoped resource mapping and the scopes each one gets. Two
// forms are accepted.
//
// A structured list, for deployments that mount a configuration file:
//
//	clientmappings:
//	  - name: controlplane
//	    scopes: [read, signaling]
//
// Or the same list as JSON, so it fits in a single environment variable. viper cannot decode a list
// of structs out of the environment on its own — AutomaticEnv is a lookup-time fallback, so
// UnmarshalKey only ever sees the raw string — hence the explicit json.Unmarshal here:
//
//	JWTLETAGENT_CLIENTMAPPINGS='[{"name":"controlplane","scopes":["read","signaling"]}]'
//
// A configured list fully replaces the built-in defaults. Returns nil when the key is unset, which
// makes the activity fall back to those defaults. Note viper treats an empty environment variable as
// unset, so JWTLETAGENT_CLIENTMAPPINGS="" also falls back.
func parseClientMappings(v *viper.Viper) ([]activity.ClientMapping, error) {
	if !v.IsSet(clientMappingsKey) {
		return nil, nil
	}

	var mappings []activity.ClientMapping
	if raw, ok := v.Get(clientMappingsKey).(string); ok {
		if err := json.Unmarshal([]byte(raw), &mappings); err != nil {
			return nil, fmt.Errorf("error reading %s: %q is not a JSON list of client mappings: %w", clientMappingsKey, raw, err)
		}
	} else if err := v.UnmarshalKey(clientMappingsKey, &mappings); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", clientMappingsKey, err)
	}

	if err := validateClientMappings(mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

// validateClientMappings rejects configurations that would leave a participant's workloads without a
// usable resource mapping. Failing at launch surfaces the misconfiguration once, instead of once per
// participant deployment.
func validateClientMappings(mappings []activity.ClientMapping) error {
	if len(mappings) == 0 {
		return fmt.Errorf("error reading %s: no client mappings configured; remove the key to use the defaults", clientMappingsKey)
	}
	seen := make(map[string]struct{}, len(mappings))
	for _, mapping := range mappings {
		if mapping.Name == "" {
			return fmt.Errorf("error reading %s: service account name is empty", clientMappingsKey)
		}
		if len(mapping.Scopes) == 0 {
			return fmt.Errorf("error reading %s: service account %q has no scopes", clientMappingsKey, mapping.Name)
		}
		if _, duplicate := seen[mapping.Name]; duplicate {
			return fmt.Errorf("error reading %s: service account %q is mapped more than once", clientMappingsKey, mapping.Name)
		}
		seen[mapping.Name] = struct{}{}
	}
	return nil
}
