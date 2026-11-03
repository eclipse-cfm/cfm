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

package health

import (
	"github.com/eclipse-cfm/cfm/assembly/routing"
	"github.com/eclipse-cfm/cfm/common/health"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
)

// HealthServiceAssembly provides the health registry and serves its checks on the liveness and readiness endpoints.
// The endpoints are unauthenticated so infrastructure probes can call them without a token.
type HealthServiceAssembly struct {
	system.DefaultServiceAssembly
	registry *health.Registry
}

func (h *HealthServiceAssembly) Name() string {
	return "Health"
}

func (h *HealthServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{health.RegistryKey}
}

func (h *HealthServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{routing.RouterKey}
}

func (h *HealthServiceAssembly) Init(ctx *system.InitContext) error {
	h.registry = health.NewRegistry()
	ctx.Registry.Register(health.RegistryKey, h.registry)
	return nil
}

// Prepare mounts the endpoints. This is deferred until all assemblies are initialized, since chi rejects middleware
// that other assemblies add to the router during Init once a route is registered.
func (h *HealthServiceAssembly) Prepare(ctx *system.InitContext) error {
	router := ctx.Registry.Resolve(routing.RouterKey).(chi.Router)
	router.Get(health.LivenessPath, h.registry.LivenessHandler())
	router.Get(health.ReadinessPath, h.registry.ReadinessHandler())
	return nil
}

// AgentAssemblies returns the assemblies that serve the health endpoints of an agent. Agents have no HTTP API of their
// own, so the server is opt-in: it is only started when the HTTP port is configured. This also keeps several agents
// from competing for a default port when they run in the same process, as they do in tests.
func AgentAssemblies(config *viper.Viper) []system.ServiceAssembly {
	if !config.IsSet(routing.PortKey) {
		return nil
	}
	return []system.ServiceAssembly{&routing.RouterServiceAssembly{}, &HealthServiceAssembly{}}
}
