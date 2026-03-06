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

package natsagent

import (
	"context"
	"fmt"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsorchestration"
)

const (
	timeout = 10 * time.Second
)

// agentServiceAssembly provides common functionality for NATS-based agents
type agentServiceAssembly struct {
	agentName        string
	activityType     string
	uri              string
	bucket           string
	streamName       string
	assemblyProvider func() []system.ServiceAssembly
	newProcessor     func(ctx *AgentContext) api.ActivityProcessor
	requires         []system.ServiceType
	system.DefaultServiceAssembly

	natsClient *natsclient.NatsClient
	cancel     context.CancelFunc
}

func (a *agentServiceAssembly) Name() string {
	return a.agentName
}

func (h *agentServiceAssembly) Requires() []system.ServiceType {
	return h.requires
}

func (a *agentServiceAssembly) Start(startCtx *system.StartContext) error {
	var err error
	a.natsClient, err = natsclient.NewNatsClient(a.uri, a.bucket)
	if err != nil {
		return fmt.Errorf("failed to create NATS client: %w", err)
	}

	if err = a.setupConsumer(a.natsClient); err != nil {
		return fmt.Errorf("failed to create setup agent consumer: %w", err)
	}

	actx := &AgentContext{
		Monitor:  startCtx.LogMonitor,
		Registry: startCtx.Registry,
		Config:   startCtx.Config,
	}

	executor := &natsorchestration.NatsActivityExecutor{
		Client:            natsclient.NewMsgClient(a.natsClient),
		StreamName:        a.streamName,
		ActivityType:      a.activityType,
		ActivityProcessor: a.newProcessor(actx),
		Monitor:           startCtx.LogMonitor,
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	return executor.Execute(ctx)
}

func (a *agentServiceAssembly) Shutdown() error {
	if a.cancel != nil {
		a.cancel()
	}

	if a.natsClient != nil {
		a.natsClient.Connection.Close()
	}
	return nil
}

func (a *agentServiceAssembly) setupConsumer(natsClient *natsclient.NatsClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stream, err := natsclient.SetupStream(ctx, natsClient, a.streamName)

	if err != nil {
		return fmt.Errorf("error setting up agent stream: %w", err)
	}

	_, err = natsclient.SetupConsumer(ctx, stream, a.activityType)

	if err != nil {
		return fmt.Errorf("error setting up agent consumer: %w", err)
	}

	return nil
}
