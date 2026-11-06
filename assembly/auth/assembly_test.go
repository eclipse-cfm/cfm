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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://idp.example.com/realms/cfm"
	testAudience = "cfm-api"
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

func enabledSettings() map[string]any {
	return map[string]any{
		jwksUrlKey:        "https://idp.example.com/jwks",
		expectedIssuerKet: testIssuer,
		authAudienceKey:   testAudience,
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

func TestInit_AuthEnabled(t *testing.T) {
	t.Run("requires an audience", func(t *testing.T) {
		for _, claim := range []string{"aud", "azp"} {
			settings := enabledSettings()
			delete(settings, authAudienceKey)
			settings[audienceClaimKey] = claim
			ctx := newInitContext(system.ProductionMode, settings)

			err := (&AuthServiceAssembly{}).Init(ctx)

			require.Error(t, err, "audience claim %s", claim)
			assert.Contains(t, err.Error(), authAudienceKey)
		}
	})

	t.Run("refuses skipping the issuer check in production mode", func(t *testing.T) {
		settings := enabledSettings()
		settings[skipIssuerCheckKey] = true
		ctx := newInitContext(system.ProductionMode, settings)

		err := (&AuthServiceAssembly{}).Init(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), skipIssuerCheckKey)
	})

	t.Run("allows skipping the issuer check in development mode", func(t *testing.T) {
		settings := enabledSettings()
		settings[skipIssuerCheckKey] = true
		ctx := newInitContext(system.DevelopmentMode, settings)

		require.NoError(t, (&AuthServiceAssembly{}).Init(ctx))
		assert.IsType(t, &oidcValidator{}, ctx.Registry.Resolve(ValidatorKey))
	})

	t.Run("registers the OIDC validator", func(t *testing.T) {
		ctx := newInitContext(system.ProductionMode, enabledSettings())

		require.NoError(t, (&AuthServiceAssembly{}).Init(ctx))
		assert.IsType(t, &oidcValidator{}, ctx.Registry.Resolve(ValidatorKey))
	})
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

func TestOidcValidator_Middleware(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	newValidator := func(audienceClaim string) *oidcValidator {
		cfg := &oidc.Config{SupportedSigningAlgs: []string{"RS256"}, SkipClientIDCheck: audienceClaim != "aud"}
		if audienceClaim == "aud" {
			cfg.ClientID = testAudience
		}
		keySet := &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{key.Public()}}
		return &oidcValidator{
			verifier:      oidc.NewVerifier(testIssuer, keySet, cfg),
			audience:      testAudience,
			audienceClaim: audienceClaim,
			monitor:       system.NoopMonitor{},
		}
	}

	serve := func(v *oidcValidator, token string) (*httptest.ResponseRecorder, *cfmauth.Claims) {
		var claims *cfmauth.Claims
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, _ = cfmauth.ClaimsFromContext(r.Context())
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		v.Middleware()(next).ServeHTTP(rec, req)
		return rec, claims
	}

	claimsWith := func(overrides map[string]any) map[string]any {
		c := map[string]any{
			"iss":   testIssuer,
			"sub":   "client-1",
			"aud":   testAudience,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"scope": "tenant-manager-api:read",
		}
		for k, v := range overrides {
			c[k] = v
		}
		return c
	}

	t.Run("accepts a valid token", func(t *testing.T) {
		rec, claims := serve(newValidator("aud"), signToken(t, key, claimsWith(nil)))

		assert.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, claims)
		assert.Equal(t, "client-1", claims.Subject)
		assert.True(t, claims.HasScope("tenant-manager-api:read"))
		assert.False(t, claims.HasScope("tenant-manager-api:write"))
	})

	t.Run("rejects a wrong audience without revealing the expected one", func(t *testing.T) {
		rec, claims := serve(newValidator("aud"), signToken(t, key, claimsWith(map[string]any{"aud": "other-client"})))

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Nil(t, claims)
		assert.Contains(t, rec.Body.String(), "invalid token")
		assert.NotContains(t, rec.Body.String(), testAudience)
		assert.NotContains(t, rec.Body.String(), "other-client")
	})

	t.Run("rejects a wrong issuer without revealing the expected one", func(t *testing.T) {
		rec, claims := serve(newValidator("aud"), signToken(t, key, claimsWith(map[string]any{"iss": "https://evil.example.com"})))

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Nil(t, claims)
		assert.NotContains(t, rec.Body.String(), testIssuer)
	})

	t.Run("checks a custom audience claim", func(t *testing.T) {
		v := newValidator("azp")

		rec, claims := serve(v, signToken(t, key, claimsWith(map[string]any{"aud": "account", "azp": testAudience})))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotNil(t, claims)

		rec, claims = serve(v, signToken(t, key, claimsWith(map[string]any{"aud": "account", "azp": "other-client"})))
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Nil(t, claims)

		rec, claims = serve(v, signToken(t, key, claimsWith(map[string]any{"aud": "account"})))
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Nil(t, claims)
	})
}

// signToken produces an RS256-signed JWT with the given claims.
func signToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	enc := base64.RawURLEncoding
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	signingInput := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signingInput + "." + enc.EncodeToString(sig)
}
