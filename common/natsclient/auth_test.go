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
	"os"
	"path/filepath"
	"testing"

	"github.com/nats-io/nkeys"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserInfoAuth_Options(t *testing.T) {
	options, err := UserInfoAuth{Username: "user", Password: "pass"}.Options()
	require.NoError(t, err)
	assert.Len(t, options, 1)

	_, err = UserInfoAuth{Username: "user"}.Options()
	assert.ErrorContains(t, err, AuthPasswordKey)

	_, err = UserInfoAuth{Password: "pass"}.Options()
	assert.ErrorContains(t, err, AuthUsernameKey)
}

func TestTokenAuth_Options(t *testing.T) {
	options, err := TokenAuth{Token: "s3cret"}.Options()
	require.NoError(t, err)
	assert.Len(t, options, 1)

	_, err = TokenAuth{}.Options()
	assert.ErrorContains(t, err, AuthTokenKey)
}

func TestNKeyAuth_Options(t *testing.T) {
	options, err := NKeyAuth{SeedFile: writeNKeySeedFile(t)}.Options()
	require.NoError(t, err)
	assert.Len(t, options, 1)

	_, err = NKeyAuth{}.Options()
	assert.ErrorContains(t, err, AuthNKeySeedFileKey)

	_, err = NKeyAuth{SeedFile: filepath.Join(t.TempDir(), "missing.nk")}.Options()
	assert.ErrorContains(t, err, "failed to load NKey seed file")
}

func TestCredentialsAuth_Options(t *testing.T) {
	credsFile := filepath.Join(t.TempDir(), "user.creds")
	require.NoError(t, os.WriteFile(credsFile, []byte("-----BEGIN NATS USER JWT-----"), 0600))

	options, err := CredentialsAuth{CredsFile: credsFile}.Options()
	require.NoError(t, err)
	assert.Len(t, options, 1)

	_, err = CredentialsAuth{}.Options()
	assert.ErrorContains(t, err, AuthCredsFileKey)

	_, err = CredentialsAuth{CredsFile: filepath.Join(t.TempDir(), "missing.creds")}.Options()
	assert.ErrorContains(t, err, "failed to read credentials file")
}

func TestAuthFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		expected AuthStrategy
	}{
		{name: "unset defaults to anonymous", settings: nil, expected: nil},
		{name: "none", settings: map[string]string{AuthMethodKey: "none"}, expected: nil},
		{
			name:     "userinfo",
			settings: map[string]string{AuthMethodKey: "userinfo", AuthUsernameKey: "user", AuthPasswordKey: "pass"},
			expected: UserInfoAuth{Username: "user", Password: "pass"},
		},
		{
			name:     "method is case-insensitive",
			settings: map[string]string{AuthMethodKey: "UserInfo", AuthUsernameKey: "user", AuthPasswordKey: "pass"},
			expected: UserInfoAuth{Username: "user", Password: "pass"},
		},
		{
			name:     "token",
			settings: map[string]string{AuthMethodKey: "token", AuthTokenKey: "s3cret"},
			expected: TokenAuth{Token: "s3cret"},
		},
		{
			name:     "nkey",
			settings: map[string]string{AuthMethodKey: "nkey", AuthNKeySeedFileKey: "/etc/nats/user.nk"},
			expected: NKeyAuth{SeedFile: "/etc/nats/user.nk"},
		},
		{
			name:     "credentials",
			settings: map[string]string{AuthMethodKey: "credentials", AuthCredsFileKey: "/etc/nats/user.creds"},
			expected: CredentialsAuth{CredsFile: "/etc/nats/user.creds"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			for key, value := range tt.settings {
				v.Set(key, value)
			}
			auth, err := AuthFromConfig(v)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, auth)
		})
	}
}

func TestAuthFromConfig_UnsupportedMethod(t *testing.T) {
	v := viper.New()
	v.Set(AuthMethodKey, "kerberos")
	_, err := AuthFromConfig(v)
	assert.ErrorContains(t, err, `unsupported NATS authentication method "kerberos"`)
}

// writeNKeySeedFile generates a user NKey pair and writes the seed to a temp file, returning its path.
func writeNKeySeedFile(t *testing.T) string {
	t.Helper()
	keyPair, err := nkeys.CreateUser()
	require.NoError(t, err)
	seed, err := keyPair.Seed()
	require.NoError(t, err)
	seedFile := filepath.Join(t.TempDir(), "user.nk")
	require.NoError(t, os.WriteFile(seedFile, seed, 0600))
	return seedFile
}
