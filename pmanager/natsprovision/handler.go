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
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
)

type natsProvisionHandler struct {
	natsclient.RetriableMessageProcessor[model.OrchestrationManifest]
}

// newNatsProvisionHandler returns the handler of the orchestration manifests that the managers send. The subject of a
// manifest decides its namespace, see natsclient.Subject, so that NATS permissions on the subjects decide which namespaces
// a manager can start orchestrations in.
func newNatsProvisionHandler(
	client natsclient.MsgClient,
	provisionManager api.ProvisionManager,
	monitor system.LogMonitor) *natsProvisionHandler {
	start := func(ctx context.Context, manifest model.OrchestrationManifest, namespace string) error {
		tracer := otel.GetTracerProvider().Tracer("cfm.pmanager.provision")
		_, span := tracer.Start(ctx, "Provision orchestration")
		defer span.End()

		var err error
		if manifest.Namespace != "" && manifest.Namespace != namespace {
			err = types.NewClientError("namespace '%s' of the manifest is not the namespace '%s' of its subject", manifest.Namespace, namespace)
		} else {
			manifest.Namespace = namespace
			_, err = provisionManager.Start(ctx, &manifest)
			span.AddEvent("Provisioning started")
		}
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
					ErrorDetail:       startErrorDetail(err, manifest.ID, monitor),
					ManifestID:        manifest.ID,
					OrchestrationType: manifest.OrchestrationType,
					Properties:        make(map[string]any),
				}
				ser, err := json.Marshal(response)
				if err != nil {
					return types.NewRecoverableError("failed to marshal response: %s", err.Error())
				}
				// the response goes to the namespace of the subject, which is the namespace of the sender
				_, err = client.Publish(ctx, natsclient.ResponseSubject(namespace, manifest.ReplyTo), ser)
				if err != nil {
					return types.NewRecoverableError("failed to publish response: %s", err.Error())
				}

				return nil // ack message back
			}
		}
		return nil
	}

	return &natsProvisionHandler{
		RetriableMessageProcessor: natsclient.RetriableMessageProcessor[model.OrchestrationManifest]{
			Client:     client,
			Monitor:    monitor,
			Processing: atomic.Bool{},
			DispatcherCtx: func(ctx context.Context, manifest model.OrchestrationManifest, message jetstream.Msg) error {
				namespace, ok := natsclient.SubjectNamespace(message.Subject())
				if !ok {
					// The consumers only receive the manifest subjects, so this is a misconfigured consumer
					monitor.Severef("Dropping orchestration manifest %s received on subject %s, which names no namespace", manifest.ID, message.Subject())
					return nil
				}
				return start(ctx, manifest, namespace)
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

// startErrorDetail describes why an orchestration could not be started. The detail is stored on the participant profile
// and returned by the Tenant Manager API, so only client errors, whose messages are written for the caller, are passed
// through. Any other error is logged under an ID that the detail refers to.
func startErrorDetail(err error, manifestID string, monitor system.LogMonitor) string {
	var clientErr types.ClientError
	if errors.As(err, &clientErr) && clientErr.IsClientError() {
		return clientErr.Error()
	}
	id := uuid.New().String()
	monitor.Severef("Orchestration for manifest %s could not be started [%s]: %v", manifestID, id, err)
	return fmt.Sprintf("orchestration could not be started [%s]", id)
}
