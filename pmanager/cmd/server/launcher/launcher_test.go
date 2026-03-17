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

package launcher

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/fixtures"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout = 30 * time.Second
	streamName  = "cfm-orchestration"
)

func TestTestAgent_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Set up NATS container
	nt, err := natsfixtures.SetupNatsContainer(ctx, "test-orchestration-bucket")

	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	natsfixtures.SetupTestStream(t, ctx, nt.Client, streamName)

	// Required agent config
	_ = os.Setenv("PM_URI", nt.URI)
	_ = os.Setenv("PM_BUCKET", "cfm-bucket")
	_ = os.Setenv("PM_STREAM", streamName)
	_ = os.Setenv("PM_HTTPPORT", strconv.Itoa(fixtures.GetRandomPort(t)))

	// Create and start the test agent
	shutdownChannel := make(chan struct{})
	go func() {
		Launch(shutdownChannel)
	}()

	// shut agent down
	shutdownChannel <- struct{}{}
}
