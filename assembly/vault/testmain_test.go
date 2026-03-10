/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package vault

import (
	"context"
	"fmt"
	"os"
	"testing"
)

var (
	sharedClient  *vaultClient
	sharedCleanup func()
)

// getTestClient returns the shared test client initialized in TestMain.
// It fails the test if the client isn't ready.
func getTestClient(t *testing.T) *vaultClient {
	t.Helper()
	if sharedClient == nil {
		t.Fatalf("test vault client not initialized")
	}
	return sharedClient
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start containers once for the entire package and create the shared client.
	client, cleanup, err := startTestEnvOnce(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start test environment: %v\n", err)
		os.Exit(1)
	}
	sharedClient = client
	sharedCleanup = cleanup

	code := m.Run()

	// Cleanup after all tests
	if sharedCleanup != nil {
		sharedCleanup()
	}
	os.Exit(code)
}
