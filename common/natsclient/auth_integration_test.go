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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const authTestTimeout = 60 * time.Second

// verifyKVRoundTrip asserts that the authenticated client is fully functional by writing and reading a KV entry.
func verifyKVRoundTrip(t *testing.T, ctx context.Context, client *natsclient.NatsClient) {
	t.Helper()
	_, err := client.KVStore.Put(ctx, "auth-test-key", []byte("auth-test-value"))
	require.NoError(t, err)
	entry, err := client.KVStore.Get(ctx, "auth-test-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("auth-test-value"), entry.Value())
}

func TestNatsClient_TokenAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainerWithAuth(ctx, "cfm-bucket",
		`authorization { token: "s3cret" }`,
		natsclient.TokenAuth{Token: "s3cret"})
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	verifyKVRoundTrip(t, ctx, nt.Client)
}

func TestNatsClient_TokenAuth_WrongToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	_, err := natsfixtures.SetupNatsContainerWithAuth(ctx, "cfm-bucket",
		`authorization { token: "s3cret" }`,
		natsclient.TokenAuth{Token: "wrong"})
	require.Error(t, err)
}

func TestNatsClient_UserInfoAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainerWithAuth(ctx, "cfm-bucket",
		`authorization { user: "cfm", password: "s3cret" }`,
		natsclient.UserInfoAuth{Username: "cfm", Password: "s3cret"})
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	verifyKVRoundTrip(t, ctx, nt.Client)
}

func TestNatsClient_UserInfoAuth_WrongCreds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	_, err := natsfixtures.SetupNatsContainerWithAuth(ctx, "cfm-bucket",
		`authorization { user: "cfm", password: "s3cret" }`,
		natsclient.UserInfoAuth{Username: "cfm", Password: "wrong_pwd"})
	require.Errorf(t, err, "foobar")
}

func TestNatsClient_NKeyAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	keyPair, err := nkeys.CreateUser()
	require.NoError(t, err)
	publicKey, err := keyPair.PublicKey()
	require.NoError(t, err)
	seed, err := keyPair.Seed()
	require.NoError(t, err)
	seedFile := filepath.Join(t.TempDir(), "user.nk")
	require.NoError(t, os.WriteFile(seedFile, seed, 0600))

	nt, err := natsfixtures.SetupNatsContainerWithAuth(ctx, "cfm-bucket",
		fmt.Sprintf(`authorization { users = [ { nkey: "%s" } ] }`, publicKey),
		natsclient.NKeyAuth{SeedFile: seedFile})
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	verifyKVRoundTrip(t, ctx, nt.Client)
}

func TestNatsClient_NKeyAuth_InvalidSeed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), authTestTimeout)
	defer cancel()

	// create key pair for the user server side
	serverSideUser, err := nkeys.CreateUser()
	require.NoError(t, err)
	publicKey, err := serverSideUser.PublicKey()
	require.NoError(t, err)

	// create DIFFERENT key pair for the user, client side
	clientSideUser, err := nkeys.CreateUser()
	require.NoError(t, err)
	seed, err := clientSideUser.Seed()
	require.NoError(t, err)
	seedFile := filepath.Join(t.TempDir(), "user.nk")
	require.NoError(t, os.WriteFile(seedFile, seed, 0600))

	_, err = natsfixtures.SetupNatsContainerWithAuth(ctx, "cfm-bucket",
		fmt.Sprintf(`authorization { users = [ { nkey: "%s" } ] }`, publicKey),
		natsclient.NKeyAuth{SeedFile: seedFile})
	require.Errorf(t, err, "invalid seed file")
}
