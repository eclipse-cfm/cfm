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

package natsorchestration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two PM replicas share the index watcher consumer: every KV update, including those written before any replica was
// running, is handled exactly once across both replicas.
func TestIndexWatcherConsumer_SharedAcrossReplicasAndCatchesUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	const bucket = "cfm-bucket"
	nt, err := natsfixtures.SetupNatsContainer(ctx, bucket)
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	const before, after = 5, 20

	// written while no replica is watching
	for i := range before {
		_, err := nt.Client.KVStore.Put(ctx, fmt.Sprintf("orch-before-%d", i), []byte("{}"))
		require.NoError(t, err)
	}

	var mu sync.Mutex
	handled := map[string]int{}
	perReplica := map[string]int{}

	loopCtx, stopLoops := context.WithCancel(ctx)
	defer stopLoops()
	for _, replica := range []string{"pm-0", "pm-1"} {
		consumer, err := SetupIndexWatcherConsumer(ctx, nt.Client, bucket)
		require.NoError(t, err)
		loop := &natsclient.FetchLoop{}
		go func() {
			_ = loop.Run(loopCtx, consumer, system.NoopMonitor{}, func(_ context.Context, msg jetstream.Msg) {
				mu.Lock()
				handled[msg.Subject()]++
				perReplica[replica]++
				mu.Unlock()
				_ = msg.Ack()
			})
		}()
	}

	for i := range after {
		_, err := nt.Client.KVStore.Put(ctx, fmt.Sprintf("orch-after-%d", i), []byte("{}"))
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(handled) == before+after
	}, processTimeout, 50*time.Millisecond, "every KV update should be handled")

	// give a duplicate delivery a chance to show up
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for subject, count := range handled {
		assert.Equal(t, 1, count, "update on %s should be handled by exactly one replica", subject)
	}
	assert.Equal(t, before+after, perReplica["pm-0"]+perReplica["pm-1"])
}
