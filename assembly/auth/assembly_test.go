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

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newInitContext(mode system.RuntimeMode, settings map[string]any) *system.InitContext {
	cfg := viper.New()
	for k, v := range settings {
		cfg.Set(k, v)
	}
	return &system.InitContext{
		StartContext: system.StartContext{
			Registry:   system.NewServiceRegistry(),
			LogMonitor: system.NoopMonitor{},
			Config:     cfg,
			Mode:       mode,
		},
	}
}

func TestInit_AuthDisabled(t *testing.T) {
	t.Run("refused in production mode", func(t *testing.T) {
		ctx := newInitContext(system.ProductionMode, map[string]any{authEnabledKey: false})

		err := (&AuthServiceAssembly{}).Init(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), allowDisabledKey)
		_, found := ctx.Registry.ResolveOptional(ValidatorKey)
		assert.False(t, found)
	})

	t.Run("allowed in production mode with explicit opt-in", func(t *testing.T) {
		ctx := newInitContext(system.ProductionMode, map[string]any{authEnabledKey: false, allowDisabledKey: true})

		require.NoError(t, (&AuthServiceAssembly{}).Init(ctx))
		assert.IsType(t, &noopValidator{}, ctx.Registry.Resolve(ValidatorKey))
	})

	for _, mode := range []system.RuntimeMode{system.DevelopmentMode, system.DebugMode} {
		t.Run("allowed in "+string(mode)+" mode", func(t *testing.T) {
			ctx := newInitContext(mode, map[string]any{authEnabledKey: false})

			require.NoError(t, (&AuthServiceAssembly{}).Init(ctx))
			assert.IsType(t, &noopValidator{}, ctx.Registry.Resolve(ValidatorKey))
		})
	}
}

func TestNoopValidator_AttachesAnonymousClaims(t *testing.T) {
	var got *cfmauth.Claims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = cfmauth.ClaimsFromContext(r.Context())
	})

	(&noopValidator{}).Middleware()(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.NotNil(t, got)
	assert.Equal(t, cfmauth.AnonymousSubject, got.Subject)
	assert.True(t, got.HasScope("tenant-manager-api:write"))
}
