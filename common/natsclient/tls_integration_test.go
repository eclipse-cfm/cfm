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

package natsclient_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/stretchr/testify/require"
)

// mutualTLSServerBlock requires TLS and verifies client certificates against the test CA.
const mutualTLSServerBlock = `
		tls {
			cert_file: "/etc/nats/certs/server.pem"
			key_file: "/etc/nats/certs/server-key.pem"
			ca_file: "/etc/nats/certs/ca.pem"
			verify: true
		}`

func TestNatsClient_MutualTLS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	pki := newTestPKI(t)
	nt, err := natsfixtures.SetupNatsContainerWithConfig(ctx, "cfm-bucket", mutualTLSServerBlock, pki.serverFiles(),
		natsclient.ClientConfig{TLS: &natsclient.TLSConfig{CAFile: pki.caFile, CertFile: pki.clientCertFile, KeyFile: pki.clientKeyFile}})
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	verifyKVRoundTrip(t, ctx, nt.Client)
}

func TestNatsClient_MutualTLS_NoClientCert(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	pki := newTestPKI(t)
	_, err := natsfixtures.SetupNatsContainerWithConfig(ctx, "cfm-bucket", mutualTLSServerBlock, pki.serverFiles(),
		natsclient.ClientConfig{TLS: &natsclient.TLSConfig{CAFile: pki.caFile}})
	require.Error(t, err)
}

func TestNatsClient_MutualTLS_UntrustedServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	pki := newTestPKI(t)
	other := newTestPKI(t)
	_, err := natsfixtures.SetupNatsContainerWithConfig(ctx, "cfm-bucket", mutualTLSServerBlock, pki.serverFiles(),
		natsclient.ClientConfig{TLS: &natsclient.TLSConfig{CAFile: other.caFile, CertFile: pki.clientCertFile, KeyFile: pki.clientKeyFile}})
	require.Error(t, err)
}

func TestNatsClient_PlaintextClientRejectedByTLSServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	pki := newTestPKI(t)
	_, err := natsfixtures.SetupNatsContainerWithConfig(ctx, "cfm-bucket", mutualTLSServerBlock, pki.serverFiles(), natsclient.ClientConfig{})
	require.Error(t, err)
}

// testPKI holds a throwaway CA with a server certificate valid for localhost and a client certificate.
type testPKI struct {
	caPEM, serverCertPEM, serverKeyPEM    []byte
	caFile, clientCertFile, clientKeyFile string
}

func (p testPKI) serverFiles() map[string][]byte {
	return map[string][]byte{
		"/etc/nats/certs/ca.pem":         p.caPEM,
		"/etc/nats/certs/server.pem":     p.serverCertPEM,
		"/etc/nats/certs/server-key.pem": p.serverKeyPEM,
	}
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cfm-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	issue := func(serial int64, cn string, usage x509.ExtKeyUsage) ([]byte, []byte) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{usage},
			DNSNames:     []string{"localhost"},
			IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
		require.NoError(t, err)
		keyDER, err := x509.MarshalECPrivateKey(key)
		require.NoError(t, err)
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	}

	pki := testPKI{caPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})}
	pki.serverCertPEM, pki.serverKeyPEM = issue(2, "nats-server", x509.ExtKeyUsageServerAuth)
	clientCertPEM, clientKeyPEM := issue(3, "cfm-component", x509.ExtKeyUsageClientAuth)

	dir := t.TempDir()
	write := func(name string, content []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, content, 0600))
		return path
	}
	pki.caFile = write("ca.pem", pki.caPEM)
	pki.clientCertFile = write("client.pem", clientCertPEM)
	pki.clientKeyFile = write("client-key.pem", clientKeyPEM)
	return pki
}
