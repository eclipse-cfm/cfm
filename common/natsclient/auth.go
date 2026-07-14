//  Copyright (c) 2026 Metaform Systems, Inc
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

package natsclient

import (
	"fmt"
	"os"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

// Configuration keys for NATS authentication, relative to the component's configuration prefix. With the standard
// environment variable mapping they translate to e.g. PM_NATS_AUTH_METHOD for the Provision Manager.
const (
	AuthMethodKey       = "nats.auth.method"
	AuthUsernameKey     = "nats.auth.username"
	AuthPasswordKey     = "nats.auth.password"
	AuthTokenKey        = "nats.auth.token"
	AuthNKeySeedFileKey = "nats.auth.nkeySeedFile"
	AuthCredsFileKey    = "nats.auth.credsFile"
)

// Supported values for AuthMethodKey.
const (
	AuthMethodNone        = "none"
	AuthMethodUserInfo    = "userinfo"
	AuthMethodToken       = "token"
	AuthMethodNKey        = "nkey"
	AuthMethodCredentials = "credentials"
)

// AuthStrategy produces the nats.Option set for one authentication mechanism. Implementations validate their inputs
// so that misconfiguration fails at startup instead of surfacing as an opaque connection error.
type AuthStrategy interface {
	Options() ([]nats.Option, error)
}

// UserInfoAuth authenticates with a username and password.
type UserInfoAuth struct {
	Username string
	Password string
}

func (a UserInfoAuth) Options() ([]nats.Option, error) {
	if a.Username == "" || a.Password == "" {
		return nil, fmt.Errorf("user/password authentication requires %s and %s to be set", AuthUsernameKey, AuthPasswordKey)
	}
	return []nats.Option{nats.UserInfo(a.Username, a.Password)}, nil
}

// TokenAuth authenticates with a static token.
type TokenAuth struct {
	Token string
}

func (a TokenAuth) Options() ([]nats.Option, error) {
	if a.Token == "" {
		return nil, fmt.Errorf("token authentication requires %s to be set", AuthTokenKey)
	}
	return []nats.Option{nats.Token(a.Token)}, nil
}

// NKeyAuth authenticates by signing the server nonce with the NKey seed read from SeedFile.
type NKeyAuth struct {
	SeedFile string
}

func (a NKeyAuth) Options() ([]nats.Option, error) {
	if a.SeedFile == "" {
		return nil, fmt.Errorf("NKey authentication requires %s to be set", AuthNKeySeedFileKey)
	}
	option, err := nats.NkeyOptionFromSeed(a.SeedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load NKey seed file %s: %w", a.SeedFile, err)
	}
	return []nats.Option{option}, nil
}

// CredentialsAuth authenticates with a JWT and NKey credentials file, used by decentralized (operator-mode) NATS
// deployments.
type CredentialsAuth struct {
	CredsFile string
}

func (a CredentialsAuth) Options() ([]nats.Option, error) {
	if a.CredsFile == "" {
		return nil, fmt.Errorf("credentials authentication requires %s to be set", AuthCredsFileKey)
	}
	// nats.UserCredentials reads the file lazily on connect, so check readability here to fail fast
	if _, err := os.Stat(a.CredsFile); err != nil {
		return nil, fmt.Errorf("failed to read credentials file %s: %w", a.CredsFile, err)
	}
	return []nats.Option{nats.UserCredentials(a.CredsFile)}, nil
}

// AuthFromConfig resolves the AuthStrategy from configuration. It returns nil when no authentication is configured
// (AuthMethodKey unset or "none"), which preserves anonymous connections. Value validation is deferred to
// AuthStrategy.Options so that all strategies report configuration errors uniformly.
func AuthFromConfig(v *viper.Viper) (AuthStrategy, error) {
	method := strings.ToLower(v.GetString(AuthMethodKey))
	switch method {
	case "", AuthMethodNone:
		return nil, nil
	case AuthMethodUserInfo:
		return UserInfoAuth{Username: v.GetString(AuthUsernameKey), Password: v.GetString(AuthPasswordKey)}, nil
	case AuthMethodToken:
		return TokenAuth{Token: v.GetString(AuthTokenKey)}, nil
	case AuthMethodNKey:
		return NKeyAuth{SeedFile: v.GetString(AuthNKeySeedFileKey)}, nil
	case AuthMethodCredentials:
		return CredentialsAuth{CredsFile: v.GetString(AuthCredsFileKey)}, nil
	default:
		return nil, fmt.Errorf("unsupported NATS authentication method %q: must be one of %s, %s, %s, %s or %s",
			method, AuthMethodNone, AuthMethodUserInfo, AuthMethodToken, AuthMethodNKey, AuthMethodCredentials)
	}
}
