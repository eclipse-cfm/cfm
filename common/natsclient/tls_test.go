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

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSConfig_Options(t *testing.T) {
	dir := t.TempDir()
	caFile := writeFile(t, dir, "ca.pem")
	certFile := writeFile(t, dir, "client.pem")
	keyFile := writeFile(t, dir, "client-key.pem")
	missing := filepath.Join(dir, "missing.pem")

	options, err := TLSConfig{}.Options()
	require.NoError(t, err)
	assert.Len(t, options, 1, "system trust store only")

	options, err = TLSConfig{CAFile: caFile}.Options()
	require.NoError(t, err)
	assert.Len(t, options, 2)

	options, err = TLSConfig{CAFile: caFile, CertFile: certFile, KeyFile: keyFile}.Options()
	require.NoError(t, err)
	assert.Len(t, options, 3)

	_, err = TLSConfig{CertFile: certFile}.Options()
	assert.ErrorContains(t, err, TLSKeyFileKey)

	_, err = TLSConfig{KeyFile: keyFile}.Options()
	assert.ErrorContains(t, err, TLSCertFileKey)

	_, err = TLSConfig{CAFile: missing}.Options()
	assert.ErrorContains(t, err, "failed to read NATS CA file")

	_, err = TLSConfig{CertFile: certFile, KeyFile: missing}.Options()
	assert.ErrorContains(t, err, "failed to read NATS client certificate file")
}

func TestTLSFromConfig(t *testing.T) {
	assert.Nil(t, TLSFromConfig(viper.New()), "TLS is not configured by default")

	v := viper.New()
	v.Set(TLSEnabledKey, true)
	assert.Equal(t, &TLSConfig{}, TLSFromConfig(v))

	v = viper.New()
	v.Set(TLSCAFileKey, "/etc/nats/ca.pem")
	v.Set(TLSCertFileKey, "/etc/nats/client.pem")
	v.Set(TLSKeyFileKey, "/etc/nats/client-key.pem")
	assert.Equal(t, &TLSConfig{CAFile: "/etc/nats/ca.pem", CertFile: "/etc/nats/client.pem", KeyFile: "/etc/nats/client-key.pem"}, TLSFromConfig(v))
}

func writeFile(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("placeholder"), 0600))
	return path
}
