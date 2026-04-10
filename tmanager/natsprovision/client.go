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
	"encoding/json"
	"sync/atomic"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
)

type natsOrchestrationClient struct {
	natsclient.RetriableMessageProcessor[model.OrchestrationResponse]
}

func newNatsOrchestrationClient(
	client natsclient.MsgClient,
	dispatcher provisionCallbackDispatcher,
	monitor system.LogMonitor) *natsOrchestrationClient {
	return &natsOrchestrationClient{
		RetriableMessageProcessor: natsclient.RetriableMessageProcessor[model.OrchestrationResponse]{
			Client:     client,
			Monitor:    monitor,
			Processing: atomic.Bool{},
			Dispatcher: func(ctx context.Context, payload model.OrchestrationResponse) error {
				tracer := otel.GetTracerProvider().Tracer("cfm.tmanager.provision")
				_, span := tracer.Start(ctx, "Dispatch orchestration response")
				defer span.End()
				err := model.Validator.Struct(payload)
				if err != nil {
					span.RecordError(err)
					return types.NewClientError("invalid response: %s", err.Error())
				}
				return dispatcher.Dispatch(ctx, payload)
			},
		},
	}
}

func (n *natsOrchestrationClient) Init(ctx context.Context, consumer jetstream.Consumer) error {
	go func() {
		err := n.ProcessLoop(ctx, consumer)
		if err != nil {
			n.Monitor.Warnf("Error Processing message: %v", err)
		}
	}()
	return nil
}

func (n *natsOrchestrationClient) Send(ctx context.Context, manifest model.OrchestrationManifest) error {
	serialized, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	_, err = n.Client.Publish(ctx, natsclient.CFMOrchestrationSubject, serialized)
	if err != nil {
		return err
	}
	return nil
}
