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
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const namespaceTestActivity = "test.namespace.activity"

// namespaceRecordingProcessor records the correlation IDs of the orchestrations it ran activities of
type namespaceRecordingProcessor struct {
	DefaultTestProcessor
	mutex sync.Mutex
	ran   []string
}

func (p *namespaceRecordingProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	correlationID, _ := ctx.Value("correlation")
	p.ran = append(p.ran, correlationID.(string)+"/"+ctx.ID())
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p *namespaceRecordingProcessor) activities() []string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]string(nil), p.ran...)
}

func namespaceTestOrchestration(namespace string, correlationID string) *api.Orchestration {
	return &api.Orchestration{
		ID:                "orchestration-1",
		Namespace:         namespace,
		CorrelationID:     correlationID,
		State:             api.OrchestrationStateInitialized,
		OrchestrationType: testOrchestrationType,
		ProcessingData:    map[string]any{"correlation": correlationID},
		OutputData:        make(map[string]any),
		Completed:         make(map[string]struct{}),
		Steps: []api.OrchestrationStep{
			{Activities: []api.Activity{{ID: "A1", Type: namespaceTestActivity}}},
			{Activities: []api.Activity{{ID: "A2", Type: namespaceTestActivity}}},
		},
	}
}

// Two namespaces run an orchestration with the same ID and activity type. Each orchestration is stored under its own
// key, its activities only reach the agent of its namespace, and its response is published in its namespace.
func TestOrchestrationsOfNamespacesAreKeptApart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-namespace-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, testStream)
	client := natsclient.NewMsgClient(nt.Client)

	responses := make(chan *nats.Msg, 4)
	subscription, err := nt.Client.Connection.Subscribe(natsclient.CFMSubjectPrefix+".*."+natsclient.CFMOrchestrationResponse, func(msg *nats.Msg) {
		responses <- msg
	})
	require.NoError(t, err)
	defer subscription.Unsubscribe()

	processors := map[string]*namespaceRecordingProcessor{}
	for _, namespace := range []string{model.DefaultNamespace, "second"} {
		natsfixtures.SetupTestNamespaceConsumer(t, ctx, stream, namespace, namespaceTestActivity)
		processors[namespace] = &namespaceRecordingProcessor{}
		executor := &NatsActivityExecutor{
			Client:            client,
			StreamName:        testStream,
			Namespace:         namespace,
			ActivityType:      namespaceTestActivity,
			ActivityProcessor: processors[namespace],
			Monitor:           system.NoopMonitor{},
		}
		require.NoError(t, executor.Execute(ctx))
	}

	orchestrator := NewNatsOrchestrator(client, system.NoopMonitor{})
	require.NoError(t, orchestrator.Execute(ctx, namespaceTestOrchestration(model.DefaultNamespace, "default-correlation")))
	require.NoError(t, orchestrator.Execute(ctx, namespaceTestOrchestration("second", "second-correlation")))

	received := map[string]model.OrchestrationResponse{}
	for len(received) < 2 {
		select {
		case msg := <-responses:
			var response model.OrchestrationResponse
			require.NoError(t, json.Unmarshal(msg.Data, &response))
			received[msg.Subject] = response
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for the responses, received %v", received)
		}
	}

	assert.Equal(t, "default-correlation", received[natsclient.CFMOrchestrationResponseSubject].CorrelationID)
	assert.Equal(t, "second-correlation", received[natsclient.ResponseSubject("second", "")].CorrelationID)
	for _, response := range received {
		assert.True(t, response.Success)
		assert.Equal(t, "orchestration-1", response.ManifestID)
	}

	assert.Equal(t, []string{"default-correlation/A1", "default-correlation/A2"}, processors[model.DefaultNamespace].activities())
	assert.Equal(t, []string{"second-correlation/A1", "second-correlation/A2"}, processors["second"].activities())

	for namespace, correlationID := range map[string]string{model.DefaultNamespace: "default-correlation", "second": "second-correlation"} {
		stored, err := orchestrator.GetOrchestration(ctx, namespace, "orchestration-1")
		require.NoError(t, err)
		require.NotNil(t, stored)
		assert.Equal(t, correlationID, stored.CorrelationID)
		assert.Equal(t, namespace, stored.Namespace)
		assert.Equal(t, api.OrchestrationStateCompleted, stored.State)
	}

	missing, err := orchestrator.GetOrchestration(ctx, "third", "orchestration-1")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

// An executor does not run an activity whose orchestration state, under the key of the executor's namespace, names
// another namespace, since the engine never writes such state.
func TestActivityExecutor_RefusesStateOfOtherNamespace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-namespace-forged-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	stream := natsfixtures.SetupTestStream(t, ctx, nt.Client, testStream)
	natsfixtures.SetupTestNamespaceConsumer(t, ctx, stream, "second", namespaceTestActivity)
	client := natsclient.NewMsgClient(nt.Client)

	forged := namespaceTestOrchestration("third", "forged")
	serialized, err := json.Marshal(forged)
	require.NoError(t, err)
	_, err = client.Update(ctx, api.OrchestrationKey("second", forged.ID), serialized, 0)
	require.NoError(t, err)

	processor := &namespaceRecordingProcessor{}
	executor := &NatsActivityExecutor{
		Client:            client,
		StreamName:        testStream,
		Namespace:         "second",
		ActivityType:      namespaceTestActivity,
		ActivityProcessor: processor,
		Monitor:           system.NoopMonitor{},
	}
	require.NoError(t, executor.Execute(ctx))
	require.NoError(t, EnqueueActivityMessages(ctx, "second", forged.ID, forged.Steps[0].Activities, client))

	time.Sleep(time.Second)
	assert.Empty(t, processor.activities())
}
