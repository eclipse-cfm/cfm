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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/system"
)

const (
	ValidatorKey system.ServiceType = "auth:Validator"

	authEnabledKey     = "auth.enabled"
	discoveryUrlKey    = "auth.discoveryUrl"
	authAudienceKey    = "auth.audience"
	audienceClaimKey   = "auth.audienceClaim"
	expectedIssuerKet  = "auth.expectedIssuer"
	jwksUrlKey         = "auth.jwksUrl"
	skipIssuerCheckKey = "auth.skipIssuerCheck"

	discoveryTimeout = 30 * time.Second
)

// AuthValidator produces a chi-compatible middleware that authenticates and enriches the request context.
type AuthValidator interface {
	Middleware() func(http.Handler) http.Handler
}

// AuthServiceAssembly wires up token validation via OIDC discovery.
// When auth.enabled is false it registers a no-op validator so dependent assemblies always find the key.
type AuthServiceAssembly struct {
	system.DefaultServiceAssembly
}

func (a *AuthServiceAssembly) Name() string {
	return "Auth"
}

func (a *AuthServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{ValidatorKey}
}

func (a *AuthServiceAssembly) Init(ctx *system.InitContext) error {
	isAuthEnabled := true // auth is enabled by default
	if ctx.Config.IsSet(authEnabledKey) {
		isAuthEnabled = ctx.Config.GetBool(authEnabledKey)
	}

	if !isAuthEnabled {
		ctx.LogMonitor.Infof("Auth is disabled — all requests will be allowed")
		ctx.Registry.Register(ValidatorKey, &noopValidator{})
		return nil
	}

	jwksURL := ctx.Config.GetString(jwksUrlKey)
	discoveryURL := ctx.Config.GetString(discoveryUrlKey)

	if jwksURL == "" && discoveryURL == "" {
		return fmt.Errorf("either auth.jwksUrl or auth.discoveryUrl must be configured when auth.enabled is true")
	}

	audience := ctx.Config.GetString(authAudienceKey)
	audienceClaim := ctx.GetConfigStrOrDefault(audienceClaimKey, "aud")
	expectedIssuer := ctx.Config.GetString(expectedIssuerKet)
	skipIssuerCheck := ctx.Config.GetBool(skipIssuerCheckKey)

	// When the IdP uses a non-standard claim for the audience (e.g. Keycloak uses azp
	// instead of aud), disable the built-in aud check and validate the claim manually
	// in the middleware instead.
	verifierCfg := &oidc.Config{
		SkipIssuerCheck:      skipIssuerCheck,
		SkipClientIDCheck:    audienceClaim != "aud",
		SupportedSigningAlgs: []string{"RS256", "EdDSA"},
	}
	if audienceClaim == "aud" {
		verifierCfg.ClientID = audience
	}

	var verifier *oidc.IDTokenVerifier

	if jwksURL != "" {
		// Explicit JWKS URL — skip OIDC discovery entirely.
		if expectedIssuer == "" {
			return fmt.Errorf("auth.expectedIssuer must be set when auth.jwksUrl is configured")
		}
		keySet := oidc.NewRemoteKeySet(context.Background(), jwksURL)
		verifier = oidc.NewVerifier(expectedIssuer, keySet, verifierCfg)
		ctx.LogMonitor.Infof("Auth initialized — jwksUrl: %s, expectedIssuer: %s, audience: %s", jwksURL, expectedIssuer, audience)
	} else {
		discoveryCtx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
		defer cancel()

		var (
			provider *oidc.Provider
			err      error
		)
		if expectedIssuer != "" {
			// discoveryUrl may be an internal endpoint whose discovery doc's issuer field
			// differs from the public URL embedded in tokens. Fetch the doc manually and
			// override the issuer so go-oidc validates tokens against expectedIssuer instead.
			provider, err = discoverWithPublicIssuer(discoveryCtx, discoveryURL, expectedIssuer)
		} else {
			provider, err = oidc.NewProvider(discoveryCtx, discoveryURL)
		}
		if err != nil {
			return fmt.Errorf("OIDC discovery failed for %q: %w", discoveryURL, err)
		}
		verifier = provider.Verifier(verifierCfg)
		ctx.LogMonitor.Infof("Auth initialized — discoveryUrl: %s, audience: %s", discoveryURL, audience)
	}

	ctx.Registry.Register(ValidatorKey, &oidcValidator{
		verifier:      verifier,
		audience:      audience,
		audienceClaim: audienceClaim,
		monitor:       ctx.LogMonitor,
	})
	return nil
}

// discoverWithPublicIssuer fetches the OIDC discovery document from discoveryURL without
// validating that its issuer field matches. It then constructs a Provider that fetches keys
// from the JWKS endpoint in the discovery document but validates iss claims against expectedIssuerKet.
//
// This is needed when the internal service URL used for discovery (e.g. an in-cluster Keycloak
// address) differs from the public URL that the IdP embeds in tokens.
func discoverWithPublicIssuer(ctx context.Context, discoveryURL, expectedIssuer string) (*oidc.Provider, error) {
	wellKnown := strings.TrimSuffix(discoveryURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, fmt.Errorf("building discovery request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery endpoint returned HTTP %d", resp.StatusCode)
	}

	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding discovery document: %w", err)
	}
	if doc.JWKSURI == "" {
		return nil, fmt.Errorf("discovery document is missing jwks_uri")
	}

	return (&oidc.ProviderConfig{
		IssuerURL: expectedIssuer,
		JWKSURL:   doc.JWKSURI,
	}).NewProvider(ctx), nil
}

// oidcValidator validates JWT bearer tokens using JWKS fetched via OIDC discovery.
type oidcValidator struct {
	verifier      *oidc.IDTokenVerifier
	audience      string // expected audience value
	audienceClaim string // claim name to check audience against (e.g. "aud" or "azp")
	monitor       system.LogMonitor
}

func (v *oidcValidator) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeUnauthorized(w, "missing or invalid bearer token")
				return
			}
			rawToken := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := v.verifier.Verify(r.Context(), rawToken)
			if err != nil {
				v.monitor.Warnw("token verification failed", "error", err)
				writeUnauthorized(w, "invalid token: "+err.Error())
				return
			}

			var rawClaims map[string]interface{}
			if err := token.Claims(&rawClaims); err != nil {
				v.monitor.Warnw("claim extraction failed", "error", err)
				writeUnauthorized(w, "invalid token claims")
				return
			}

			if v.audienceClaim != "aud" && v.audience != "" {
				if val, _ := rawClaims[v.audienceClaim].(string); val != v.audience {
					v.monitor.Warnw("audience claim mismatch", "claim", v.audienceClaim, "expected", v.audience, "got", val)
					writeUnauthorized(w, "invalid token audience")
					return
				}
			}

			claims := &cfmauth.Claims{
				Subject: token.Subject,
				Scopes:  extractScopes(rawClaims),
			}

			ctx := context.WithValue(r.Context(), cfmauth.ContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// noopValidator is used when auth is disabled; its middleware passes every request through.
type noopValidator struct{}

func (n *noopValidator) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

// extractScopes parses the standard space-separated "scope" claim (RFC 9068).
func extractScopes(raw map[string]interface{}) []string {
	scopeStr, ok := raw["scope"].(string)
	if !ok || scopeStr == "" {
		return nil
	}
	return strings.Fields(scopeStr)
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprintf(w, `{"error":"Unauthorized","message": %q}`, message)
}
