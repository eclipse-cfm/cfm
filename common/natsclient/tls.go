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

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

// Configuration keys for NATS transport security, relative to the component's configuration prefix. With the standard
// environment variable mapping they translate to e.g. PM_NATS_TLS_CA for the Provision Manager.
const (
	TLSEnabledKey  = "nats.tls.enabled"
	TLSCAFileKey   = "nats.tls.ca"
	TLSCertFileKey = "nats.tls.cert"
	TLSKeyFileKey  = "nats.tls.key"
)

// TLSConfig configures TLS for the NATS connection. With no CA file the system trust store verifies the server. A
// client certificate and key enable mutual TLS.
type TLSConfig struct {
	CAFile   string
	CertFile string
	KeyFile  string
}

func (c TLSConfig) Options() ([]nats.Option, error) {
	if (c.CertFile == "") != (c.KeyFile == "") {
		return nil, fmt.Errorf("mutual TLS requires both %s and %s to be set", TLSCertFileKey, TLSKeyFileKey)
	}
	options := []nats.Option{nats.Secure()}
	if c.CAFile != "" {
		// nats.RootCAs reads the file lazily on connect, so check readability here to fail fast
		if _, err := os.Stat(c.CAFile); err != nil {
			return nil, fmt.Errorf("failed to read NATS CA file %s: %w", c.CAFile, err)
		}
		options = append(options, nats.RootCAs(c.CAFile))
	}
	if c.CertFile != "" {
		for _, file := range []string{c.CertFile, c.KeyFile} {
			if _, err := os.Stat(file); err != nil {
				return nil, fmt.Errorf("failed to read NATS client certificate file %s: %w", file, err)
			}
		}
		options = append(options, nats.ClientCert(c.CertFile, c.KeyFile))
	}
	return options, nil
}

// TLSFromConfig resolves the TLS configuration. TLS is enabled when TLSEnabledKey is true or any of the CA,
// certificate or key files is configured; otherwise nil is returned and the connection follows the URL scheme
// (tls:// still enforces TLS against the system trust store).
func TLSFromConfig(v *viper.Viper) *TLSConfig {
	config := TLSConfig{
		CAFile:   v.GetString(TLSCAFileKey),
		CertFile: v.GetString(TLSCertFileKey),
		KeyFile:  v.GetString(TLSKeyFileKey),
	}
	if !v.GetBool(TLSEnabledKey) && config == (TLSConfig{}) {
		return nil
	}
	return &config
}
