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

	"github.com/eclipse-cfm/cfm/common/health"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	setupStreamKey = "setupStream"

	// IndexWatcherConsumer is the durable consumer through which the PM replicas share the orchestration updates written
	// to the KV bucket. Each update is delivered to a single replica.
	IndexWatcherConsumer = "pmanager-index-watcher"
)

type natsOrchestratorServiceAssembly struct {
	clientConfig natsclient.ClientConfig
	streamName   string
	natsClient   *natsclient.NatsClient
	system.DefaultServiceAssembly
	processCancel context.CancelFunc
	watcherLoop   natsclient.FetchLoop
}

func NewOrchestratorServiceAssembly(clientConfig natsclient.ClientConfig, streamName string) system.ServiceAssembly {
	return &natsOrchestratorServiceAssembly{
		clientConfig: clientConfig,
		streamName:   streamName,
	}
}

func (a *natsOrchestratorServiceAssembly) Name() string {
	return "NATs Provisioning"
}

func (a *natsOrchestratorServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{api.OrchestratorKey, natsclient.NatsClientKey}
}

func (a *natsOrchestratorServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{api.OrchestrationIndexKey, store.TransactionContextKey}
}

func (a *natsOrchestratorServiceAssembly) Init(ctx *system.InitContext) error {
	natsClient, err := natsclient.NewNatsClient(a.clientConfig)
	if err != nil {
		return err
	}

	a.natsClient = natsClient
	ctx.Registry.Register(natsclient.NatsClientKey, natsClient)

	natsContext := context.Background()
	defer natsContext.Done()

	setupStream := true
	if ctx.Config.IsSet(setupStreamKey) {
		setupStream = ctx.Config.GetBool(setupStreamKey)
	}

	if setupStream {
		_, err = natsclient.SetupStream(natsContext, natsClient, a.streamName)
		if err != nil {
			return fmt.Errorf("error initializing NATS stream: %w", err)
		}
	}

	client := natsclient.NewMsgClient(natsClient)
	orchestrator := NewNatsOrchestrator(client, ctx.LogMonitor)
	ctx.Registry.Register(api.OrchestratorKey, orchestrator)

	return nil
}

func (a *natsOrchestratorServiceAssembly) Prepare(ctx *system.InitContext) error {
	index := ctx.Registry.Resolve(api.OrchestrationIndexKey).(store.EntityStore[*api.OrchestrationEntry])
	trxContext := ctx.Registry.Resolve(store.TransactionContextKey).(store.TransactionContext)
	// have to instantiate the watcher here, otherwise the provision manager would cause a cyclic dependency
	provisionManager := ctx.Registry.Resolve(api.ProvisionManagerKey).(api.ProvisionManager)
	definitionManager := ctx.Registry.Resolve(api.DefinitionManagerKey).(api.DefinitionManager)

	watcher := &OrchestrationIndexWatcher{
		index:             index,
		trxContext:        trxContext,
		monitor:           ctx.LogMonitor,
		provisionManager:  provisionManager,
		definitionManager: definitionManager,
	}
	health.RegisterOptional(ctx.Registry, func(r *health.Registry) {
		r.AddReadiness("nats", health.NatsConnectionCheck(a.natsClient.Connection))
	})

	bucket := a.clientConfig.Bucket
	setupConsumer := func(ctx context.Context) (jetstream.Consumer, error) {
		return SetupIndexWatcherConsumer(ctx, a.natsClient, bucket)
	}
	consumer, err := setupConsumer(context.Background())
	if err != nil {
		return err
	}
	a.watcherLoop.Recreate = setupConsumer
	health.RegisterOptional(ctx.Registry, func(r *health.Registry) {
		r.AddConsumerLoop("consumer:"+IndexWatcherConsumer, &a.watcherLoop)
	})

	loopCtx, cancel := context.WithCancel(context.Background())
	a.processCancel = cancel
	go func() {
		_ = a.watcherLoop.Run(loopCtx, consumer, ctx.LogMonitor, func(_ context.Context, msg jetstream.Msg) {
			watcher.onMessage(msg.Data(), msg)
		})
	}()
	return nil
}

// SetupIndexWatcherConsumer creates or updates the durable consumer that feeds the orchestration index watcher. It reads
// the stream backing the KV bucket, so updates written while no PM replica is running are delivered once one starts.
// On first creation it delivers the latest value of every key, which (re)builds the index.
func SetupIndexWatcherConsumer(ctx context.Context, client *natsclient.NatsClient, bucket string) (jetstream.Consumer, error) {
	stream, err := natsclient.GetExistingStream(ctx, client, "KV_"+bucket)
	if err != nil {
		return nil, err
	}
	return stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       IndexWatcherConsumer,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		FilterSubject: "$KV." + bucket + ".>",
	})
}

func (a *natsOrchestratorServiceAssembly) Shutdown() error {
	if a.processCancel != nil {
		a.processCancel()
	}
	if a.natsClient != nil {
		a.natsClient.Connection.Close()
	}
	return nil
}
