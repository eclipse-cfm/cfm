//  Copyright (c) 2026 Metaform Systems, Inc
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

package lifecycleagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go/jetstream"
)

const timeout = 10 * time.Second

// eventAgentServiceAssembly wires a lifecycle agent into the service lifecycle: on Start it connects to NATS, ensures
// the configured stream and durable consumer exist, and runs a RetriableMessageProcessor that decodes each message and
// dispatches it to the agent's EventProcessor.
type eventAgentServiceAssembly[T any] struct {
	agentName    string
	clientConfig natsclient.ClientConfig
	streamName   string
	subjects     []string
	createStream bool
	newProcessor func(ctx *AgentContext) EventProcessor[T]
	requires     []system.ServiceType
	system.DefaultServiceAssembly

	natsClient *natsclient.NatsClient
	cancel     context.CancelFunc
}

func (a *eventAgentServiceAssembly[T]) Name() string {
	return a.agentName
}

func (a *eventAgentServiceAssembly[T]) Requires() []system.ServiceType {
	return a.requires
}

func (a *eventAgentServiceAssembly[T]) Start(startCtx *system.StartContext) error {
	var err error
	a.natsClient, err = natsclient.NewNatsClient(a.clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create NATS client: %w", err)
	}

	consumer, err := a.setupConsumer(a.natsClient)
	if err != nil {
		return fmt.Errorf("failed to set up agent consumer: %w", err)
	}

	actx := &AgentContext{
		Monitor:  startCtx.LogMonitor,
		Registry: startCtx.Registry,
		Config:   startCtx.Config,
	}

	processor := a.newProcessor(actx)

	msgProcessor := &natsclient.RetriableMessageProcessor[T]{
		Client:  natsclient.NewMsgClient(a.natsClient),
		Monitor: startCtx.LogMonitor,
		DispatcherCtx: func(ctx context.Context, payload T, message jetstream.Msg) error {
			return processor.Process(ctx, EventContext[T]{
				Payload: payload,
				Subject: message.Subject(),
				Header:  message.Headers(),
				Raw:     message.Data(),
			})
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	go func() {
		if err := msgProcessor.ProcessLoop(ctx, consumer); err != nil {
			startCtx.LogMonitor.Warnf("Error processing lifecycle agent messages: %v", err)
		}
	}()

	return nil
}

func (a *eventAgentServiceAssembly[T]) Shutdown() error {
	if a.cancel != nil {
		a.cancel()
	}

	if a.natsClient != nil {
		a.natsClient.Connection.Close()
	}
	return nil
}

func (a *eventAgentServiceAssembly[T]) setupConsumer(natsClient *natsclient.NatsClient) (jetstream.Consumer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var stream jetstream.Stream
	var err error
	if a.createStream {
		// Create the shared event stream with the agent's subjects if it does not already exist.
		stream, err = natsclient.GetStream(ctx, natsClient, a.streamName, a.subjects)
	} else {
		// The shared event stream is managed externally and must already exist; fail fast if it is not
		// present so the agent is restarted until it is.
		stream, err = natsclient.GetExistingStream(ctx, natsClient, a.streamName)
	}
	if err != nil {
		return nil, fmt.Errorf("error resolving agent stream: %w", err)
	}

	consumer, err := natsclient.SetupMultiSubjectConsumer(ctx, stream, durableName(a.agentName), a.subjects)
	if err != nil {
		return nil, fmt.Errorf("error setting up agent consumer: %w", err)
	}

	return consumer, nil
}

// durableName derives a NATS-safe durable consumer name from the agent name. NATS durable names cannot contain spaces,
// dots, or other separators, so those are replaced with dashes.
func durableName(agentName string) string {
	replacer := strings.NewReplacer(" ", "-", ".", "-", "\t", "-", "/", "-", "*", "-", ">", "-")
	return strings.ToLower(replacer.Replace(agentName))
}
