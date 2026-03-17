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
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/natsorchestration"

	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout  = 30 * time.Second
	pollInterval = 100 * time.Millisecond
	streamName   = "cfm-activity"
)

func TestTestAgent_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Set up NATS container
	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")

	require.NoError(t, err)

	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	natsfixtures.SetupTestStream(t, ctx, nt.Client, streamName)

	// Set up an orchestration for the test agent to process
	orchestration := api.Orchestration{
		ID:             "test-agent-orchestration",
		State:          api.OrchestrationStateRunning,
		Completed:      make(map[string]struct{}),
		ProcessingData: make(map[string]any),
		OutputData:     make(map[string]any),
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{ID: "test-activity", Type: "test.activity"},
				},
			},
		},
	}

	// Required agent config
	_ = os.Setenv("TESTAGENT_URI", nt.URI)
	_ = os.Setenv("TESTAGENT_BUCKET", "cfm-bucket")
	_ = os.Setenv("TESTAGENT_STREAM", streamName)

	// Create and start the test agent
	shutdownChannel := make(chan struct{})
	go func() {
		Launch(shutdownChannel)
	}()

	// Submit orchestration
	adapter := natsclient.NewMsgClient(nt.Client)
	logMonitor := runtime.LoadLogMonitor("test-agent", system.DevelopmentMode)
	orchestrator := natsorchestration.NewNatsOrchestrator(adapter, logMonitor)

	err = orchestrator.Execute(ctx, &orchestration)
	require.NoError(t, err)

	// Wait for the activity to be processed
	assert.Eventually(t, func() bool {
		updatedOrchestration, _, err := natsorchestration.ReadOrchestration(ctx, orchestration.ID, adapter)
		require.NoError(t, err)
		return updatedOrchestration.State == api.OrchestrationStateCompleted
	}, testTimeout, pollInterval, "Activity should be processed")

	// shut agent down
	shutdownChannel <- struct{}{}
}
