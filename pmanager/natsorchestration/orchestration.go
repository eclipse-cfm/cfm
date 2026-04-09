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

// Package natsorchestration implements a NATS-based orchestrator.
package natsorchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
)

// NatsOrchestrator is responsible for executing an orchestration using NATS for reliable messaging. For each
// activity, a message is published to a durable queue based on the activity type. Activity messages are then dequeued
// and reliably processed by a NatsActivityExecutor that handles the activity type.
type NatsOrchestrator struct {
	Client     natsclient.MsgClient
	index      store.EntityStore[*api.OrchestrationEntry]
	trxContext store.TransactionContext
	monitor    system.LogMonitor
}

func NewNatsOrchestrator(
	client natsclient.MsgClient,
	monitor system.LogMonitor) *NatsOrchestrator {
	return &NatsOrchestrator{Client: client, monitor: monitor}
}

func (o *NatsOrchestrator) GetOrchestration(ctx context.Context, id string) (*api.Orchestration, error) {
	orchestration, _, err := ReadOrchestration(ctx, id, o.Client)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			// Doesn't exist return nil
			return nil, nil
		}
		// Return other errors
		return nil, fmt.Errorf("error reading orchestration %s: %w", id, err)
	}
	return &orchestration, nil
}

// Execute asynchronously executes the given orchestration by dispatching messages to durable activity
// queues, where they can be dequeued and reliably processed by NatsActivityExecutors.
//
// A Jetstream KV entry is used to maintain durable state and is updated as the orchestration progresses. This
// state is passed to the executors, which access and update it.

func (o *NatsOrchestrator) Execute(ctx context.Context, orchestration *api.Orchestration) error {
	// TODO validate orchestration - this should include a check to see if there are no steps or steps with no activities

	_, span := otel.GetTracerProvider().Tracer("nats.orchestrator").Start(ctx, "nats.execute_orchestration")
	defer span.End()

	serializedOrchestration, err := json.Marshal(orchestration)
	if err != nil {
		return fmt.Errorf("error marshalling orchestration: %w", err)
	}

	// Use update to check if the orchestration already exists
	_, err = o.Client.Update(ctx, orchestration.ID, serializedOrchestration, 0)
	if err != nil {
		var jsErr *jetstream.APIError
		if errors.As(err, &jsErr) {
			if jsErr.APIError().ErrorCode == jetstream.JSErrCodeStreamWrongLastSequence {
				// Orchestration already exists, return
				return nil
			}
		}
		return fmt.Errorf("error storing orchestration: %w", err)
	}

	activities := getInitialActivities(orchestration)
	if len(activities) == 0 {
		return fmt.Errorf("orchestration has no activities: %s", orchestration.ID)
	}
	span.AddEvent("Activity messages enqueued")
	err = EnqueueActivityMessages(ctx, orchestration.ID, activities, o.Client)
	if err != nil {
		return err
	}
	return nil
}

// Returns the initial activities for the given orchestration.
// If the orchestration has no activities, an empty list is returned.
func getInitialActivities(orchestration *api.Orchestration) []api.Activity {
	for _, step := range orchestration.Steps {
		if len(step.Activities) > 0 {
			return step.Activities
		}
	}
	return []api.Activity{}
}
