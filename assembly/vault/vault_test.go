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

package vault

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreAndResolveSecret(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestFixtures(ctx, t)
	defer cleanup()

	newSecretPath := "new-test-secret"
	newSecretValue := "new-secret-value"
	err := client.StoreSecret(ctx, newSecretPath, newSecretValue)
	require.NoError(t, err, "Failed to store secret")

	retrievedValue, err := client.ResolveSecret(ctx, newSecretPath)
	require.NoError(t, err, "Failed to resolve secret")

	assert.Equalf(t, newSecretValue, retrievedValue, "Expected secret value %q, got %q", newSecretValue, retrievedValue)
}

func TestDeleteSecret_WithSoftDelete(t *testing.T) {
	ctx := context.Background()

	client, cleanup := setupTestFixtures(ctx, t)
	client.softDelete = true
	defer cleanup()

	secretToDelete := "secret-to-delete"
	err := client.StoreSecret(ctx, secretToDelete, "delete-me")
	require.NoError(t, err, "Failed to store secret")

	err = client.DeleteSecret(ctx, secretToDelete)
	require.NoError(t, err, "Failed to delete secret")

	// Try to retrieve the deleted secret (should fail)
	_, err = client.ResolveSecret(ctx, secretToDelete)
	require.NotNil(t, err, "Expected error when retrieving deleted secret, but got none")
}

func TestDeleteSecret_NoSoftDelete(t *testing.T) {
	ctx := context.Background()

	client, cleanup := setupTestFixtures(ctx, t)
	defer cleanup()

	secretToDelete := "secret-to-delete"
	err := client.StoreSecret(ctx, secretToDelete, "delete-me")
	require.NoError(t, err, "Failed to store secret")

	err = client.DeleteSecret(ctx, secretToDelete)
	require.NoError(t, err, "Failed to delete secret")

	// Try to retrieve the deleted secret (should fail)
	_, err = client.ResolveSecret(ctx, secretToDelete)
	require.NotNil(t, err, "Expected error when retrieving deleted secret, but got none")
}

func Test_TokenRenewal(t *testing.T) {
	client, cleanup := setupTestFixtures(context.Background(), t)
	defer cleanup()
	defer client.Close()

	go client.renewTokenPeriodically(10 * time.Millisecond)

	require.Eventually(t, func() bool {
		return !client.lastRenew.IsZero()
	}, 5*time.Second, 10*time.Millisecond, "Token renewal did not occur within timeout")
}
