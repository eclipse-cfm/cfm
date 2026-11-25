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

// The sample manager starts one orchestration and waits for its response. It shows what a manager does: it sends an
// orchestration manifest to the Provision Manager on the subject of its namespace, and consumes the orchestration
// responses of its namespace. The Tenant Manager of cfm-dataspace is a manager of this kind. See samples/README.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	uri := flag.String("uri", "nats://localhost:4222", "URI of the NATS server")
	bucket := flag.String("bucket", "cfm-bucket", "JetStream key-value bucket of the Provision Manager")
	streamName := flag.String("stream", "cfm-stream", "JetStream stream of the Provision Manager")
	namespace := flag.String("namespace", model.DefaultNamespace, "namespace of the manager")
	orchestrationType := flag.String("type", "sample.workspace.deploy", "orchestration type to start")
	workspace := flag.String("workspace", "acme", "workspace to deploy or dispose")
	flag.Parse()

	if err := run(*uri, *bucket, *streamName, *namespace, *orchestrationType, *workspace); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(uri string, bucket string, streamName string, namespace string, orchestrationType string, workspace string) error {
	if err := model.ValidateNamespace(namespace); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	client, err := natsclient.NewNatsClient(natsclient.ClientConfig{URL: uri, Bucket: bucket})
	if err != nil {
		return err
	}
	defer client.Close()

	stream, err := natsclient.SetupStream(ctx, client, streamName)
	if err != nil {
		return err
	}

	// The durable consumer of the orchestration responses of the namespace: <namespace>~cfm-orchestration-response on
	// the subject event.<namespace>.cfm-orchestration-response
	consumer, err := natsclient.SetupNamespaceConsumer(ctx, stream, namespace, natsclient.CFMOrchestrationResponse)
	if err != nil {
		return err
	}

	// The payload carries the fields that the schema of the orchestration template declares. The subject decides the
	// namespace of the orchestration, so the manifest does not need to name it.
	manifest := model.OrchestrationManifest{
		ID:                uuid.NewString(),
		CorrelationID:     workspace,
		OrchestrationType: model.OrchestrationType(orchestrationType),
		Payload: map[string]any{
			"workspace": workspace,
			"region":    "eu-central",
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	// event.<namespace>.cfm-orchestration
	subject := natsclient.Subject(namespace, natsclient.CFMOrchestration)
	if _, err = client.JetStream.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("failed to publish the manifest to %s: %w", subject, err)
	}
	fmt.Printf("Started orchestration %s of type %s in namespace %s\n", manifest.ID, orchestrationType, namespace)

	for ctx.Err() == nil {
		message, err := consumer.Next(jetstream.FetchMaxWait(2 * time.Second))
		if err != nil {
			continue // no response yet
		}
		var response model.OrchestrationResponse
		if err = json.Unmarshal(message.Data(), &response); err != nil {
			return err
		}
		if err = message.Ack(); err != nil {
			return err
		}
		if response.ManifestID != manifest.ID {
			continue // the response of another orchestration of the namespace
		}
		if !response.Success {
			return fmt.Errorf("orchestration %s failed: %s", response.ManifestID, response.ErrorDetail)
		}
		fmt.Printf("Orchestration %s completed with the properties %v\n", response.ManifestID, response.Properties)
		return nil
	}
	return fmt.Errorf("no response for orchestration %s: %w", manifest.ID, ctx.Err())
}
