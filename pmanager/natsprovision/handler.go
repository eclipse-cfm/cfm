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
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/trace"
)

type natsProvisionHandler struct {
	natsclient.RetriableMessageProcessor[model.OrchestrationManifest]
}

func newNatsProvisionHandler(
	client natsclient.MsgClient,
	provisionManager api.ProvisionManager,
	monitor system.LogMonitor,
	tracer trace.Tracer) *natsProvisionHandler {
	return &natsProvisionHandler{
		RetriableMessageProcessor: natsclient.RetriableMessageProcessor[model.OrchestrationManifest]{
			Client:     client,
			Monitor:    monitor,
			Processing: atomic.Bool{},
			Tracer:     tracer,
			Dispatcher: func(ctx context.Context, manifest model.OrchestrationManifest) error {
				_, span := tracer.Start(ctx, "Provision orchestration")
				defer span.End()

				_, err := provisionManager.Start(ctx, &manifest)
				span.AddEvent("Provisioning started")
				if err != nil {
					span.RecordError(err)
					switch {
					case types.IsRecoverable(err):
						// Return error to NAK the message and retry
						return err
					default:

						// return error response
						response := &model.OrchestrationResponse{
							ID:                uuid.New().String(),
							Success:           false,
							CorrelationID:     manifest.CorrelationID,
							ErrorDetail:       err.Error(),
							ManifestID:        manifest.ID,
							OrchestrationType: manifest.OrchestrationType,
							Properties:        make(map[string]any),
						}
						ser, err := json.Marshal(response)
						if err != nil {
							return types.NewRecoverableError("failed to marshal response: %s", err.Error())
						}
						_, err = client.Publish(ctx, natsclient.CFMOrchestrationResponseSubject, ser)
						if err != nil {
							return types.NewRecoverableError("failed to publish response: %s", err.Error())
						}

						return nil // ack message back
					}
				}
				return nil
			},
		},
	}
}

func (n *natsProvisionHandler) Init(ctx context.Context, consumer jetstream.Consumer) error {
	go func() {
		err := n.ProcessLoop(ctx, consumer)
		if err != nil {
			n.Monitor.Warnf("Error processing message: %v", err)
		}
	}()
	return nil
}
