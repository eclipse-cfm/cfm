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
	"github.com/eclipse-cfm/cfm/pmanager/api"
)

type natsProvisionServiceAssembly struct {
	streamName       string
	natsClient       *natsclient.NatsClient
	provisionHandler *natsProvisionHandler
	system.DefaultServiceAssembly
	processCancel context.CancelFunc
}

func NewProvisionServiceAssembly(streamName string) system.ServiceAssembly {
	return &natsProvisionServiceAssembly{
		streamName: streamName,
	}
}

func (a *natsProvisionServiceAssembly) Name() string {
	return "NATs Provision"
}

func (a *natsProvisionServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{api.ProvisionManagerKey, natsclient.NatsClientKey}
}

func (a *natsProvisionServiceAssembly) Init(ctx *system.InitContext) error {

	a.natsClient = ctx.Registry.Resolve(natsclient.NatsClientKey).(*natsclient.NatsClient)

	natsContext := context.Background()
	defer natsContext.Done()

	provisionManager := ctx.Registry.Resolve(api.ProvisionManagerKey).(api.ProvisionManager)
	client := natsclient.NewMsgClient(a.natsClient)
	a.provisionHandler = newNatsProvisionHandler(client, provisionManager, ctx.LogMonitor)

	return nil
}

func (a *natsProvisionServiceAssembly) Start(_ *system.StartContext) error {
	var ctx context.Context
	natsContext := context.Background()
	defer natsContext.Done()

	stream, err := natsclient.SetupStream(natsContext, a.natsClient, a.streamName)
	if err != nil {
		return fmt.Errorf("error initializing NATS stream: %w", err)
	}

	consumer, err := natsclient.SetupConsumer(natsContext, stream, natsclient.CFMOrchestration)
	if err != nil {
		return fmt.Errorf("error initializing NATS orchestration manifest consumer: %w", err)
	}

	ctx, a.processCancel = context.WithCancel(context.Background())
	return a.provisionHandler.Init(ctx, consumer)
}

func (a *natsProvisionServiceAssembly) Shutdown() error {
	if a.processCancel != nil {
		a.processCancel()
	}
	if a.natsClient != nil {
		a.natsClient.Connection.Close()
	}
	return nil
}
