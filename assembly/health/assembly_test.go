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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eclipse-cfm/cfm/assembly/routing"
	"github.com/eclipse-cfm/cfm/common/health"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// middlewareAssembly adds router middleware during Init, as the TM and PM handler assemblies do, and contributes a
// failing readiness check.
type middlewareAssembly struct {
	system.DefaultServiceAssembly
	router *chi.Mux
}

func (m *middlewareAssembly) Name() string { return "Middleware" }

func (m *middlewareAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{routing.RouterKey}
}

func (m *middlewareAssembly) Init(ctx *system.InitContext) error {
	m.router = ctx.Registry.Resolve(routing.RouterKey).(*chi.Mux)
	m.router.Use(middleware.Recoverer)
	return nil
}

func (m *middlewareAssembly) Start(ctx *system.StartContext) error {
	health.RegisterOptional(ctx.Registry, func(r *health.Registry) {
		r.AddReadiness("db", func(context.Context) error { return errors.New("unreachable") })
	})
	return nil
}

func TestHealthServiceAssembly_ServesEndpoints(t *testing.T) {
	assembler := system.NewServiceAssembler(system.NoopMonitor{}, viper.New(), system.ProductionMode)
	assembler.Register(&routing.RouterServiceAssembly{})
	assembler.Register(&HealthServiceAssembly{})
	mw := &middlewareAssembly{}
	assembler.Register(mw)

	require.NotPanics(t, func() { require.NoError(t, assembler.Assemble()) })
	t.Cleanup(func() { _ = assembler.Shutdown() })

	recorder := httptest.NewRecorder()
	mw.router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, health.LivenessPath, nil))
	assert.Equal(t, http.StatusOK, recorder.Code)

	recorder = httptest.NewRecorder()
	mw.router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, health.ReadinessPath, nil))
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.JSONEq(t, `{"status":"DOWN","checks":{"db":"DOWN"}}`, recorder.Body.String())
}

func TestAgentAssemblies_OptIn(t *testing.T) {
	config := viper.New()
	assert.Empty(t, AgentAssemblies(config), "agents only serve health endpoints when a port is configured")

	config.Set(routing.PortKey, 8080)
	assert.Len(t, AgentAssemblies(config), 2)
}
