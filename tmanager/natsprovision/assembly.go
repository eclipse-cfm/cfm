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

package natsprovision

import (
	"context"
	"fmt"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

type natsOrchestrationServiceAssembly struct {
	uri                 string
	bucket              string
	streamName          string
	natsClient          *natsclient.NatsClient
	orchestrationClient *natsOrchestrationClient
	processCancel       context.CancelFunc

	system.DefaultServiceAssembly
}

func NewNatsOrchestrationServiceAssembly(uri string, bucket string, streamName string) system.ServiceAssembly {
	return &natsOrchestrationServiceAssembly{
		uri:        uri,
		bucket:     bucket,
		streamName: streamName,
	}
}

func (a *natsOrchestrationServiceAssembly) Name() string {
	return "NATs Orchestration Client"
}

func (a *natsOrchestrationServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{api.ProvisionClientKey}
}

func (d *natsOrchestrationServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{}
}

func (a *natsOrchestrationServiceAssembly) Init(ctx *system.InitContext) error {
	natsClient, err := natsclient.NewNatsClient(a.uri, a.bucket)
	if err != nil {
		return err
	}

	a.natsClient = natsClient

	dispatcher := newProvisionCallbackService()
	ctx.Registry.Register(api.ProvisionHandlerRegistryKey, dispatcher)

	client := natsclient.NewMsgClient(natsClient)
	a.orchestrationClient = newNatsOrchestrationClient(client, dispatcher, ctx.LogMonitor)
	ctx.Registry.Register(api.ProvisionClientKey, a.orchestrationClient)

	return nil
}

func (a *natsOrchestrationServiceAssembly) Start(_ *system.StartContext) error {
	var ctx context.Context
	natsContext := context.Background()
	defer natsContext.Done()

	stream, err := natsclient.SetupStream(natsContext, a.natsClient, a.streamName)
	if err != nil {
		return fmt.Errorf("error initializing NATS stream: %w", err)
	}

	consumer, err := natsclient.SetupConsumer(natsContext, stream, natsclient.CFMOrchestrationResponse)
	if err != nil {
		return fmt.Errorf("error initializing NATS orchestration consumer: %w", err)
	}

	ctx, a.processCancel = context.WithCancel(context.Background())

	return a.orchestrationClient.Init(ctx, consumer)
}

func (a *natsOrchestrationServiceAssembly) Shutdown() error {
	if a.processCancel != nil {
		a.processCancel()
	}
	if a.natsClient != nil {
		a.natsClient.Connection.Close()
	}
	return nil
}
