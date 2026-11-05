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
	"sync"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var (
	orchMetricsOnce       sync.Once
	orchStartedCounter    metric.Int64Counter
	orchCompletedCounter  metric.Int64Counter
	orchDurationHistogram metric.Float64Histogram
)

func initOrchMetrics() {
	orchMetricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("cfm.pmanager.orchestrator")
		orchStartedCounter, _ = meter.Int64Counter("cfm.orchestration.started.total")
		orchCompletedCounter, _ = meter.Int64Counter("cfm.orchestration.completed.total")
		orchDurationHistogram, _ = meter.Float64Histogram("cfm.orchestration.duration",
			metric.WithUnit("ms"))
	})
}

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
//
// Storing the orchestration and publishing its first activities are two separate steps, so a failure or a crash in
// between leaves an orchestration that is stored but was never started. The orchestration is marked running only once
// its first activities are published, and calling Execute again for an orchestration that is still initialized
// publishes them again. Execute can therefore be retried until it succeeds. An error from publishing the activities is
// recoverable for that reason.
func (o *NatsOrchestrator) Execute(ctx context.Context, orchestration *api.Orchestration) error {
	// TODO validate orchestration - this should include a check to see if there are no steps or steps with no activities
	if len(getInitialActivities(orchestration)) == 0 {
		return fmt.Errorf("orchestration has no activities: %s", orchestration.ID)
	}

	ctx, span := otel.GetTracerProvider().Tracer("cfm.pmanager.orchestrator").Start(ctx, "nats.execute_orchestration")
	defer span.End()

	// Capture the originating trace context so reactive flows that lack an inbound message to extract from
	// (notably auto-compensation launched later from the KV watcher) can link back to this trace.
	if orchestration.OriginTraceContext == nil {
		carrier := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, carrier)
		if len(carrier) > 0 {
			orchestration.OriginTraceContext = carrier
		}
	}

	serializedOrchestration, err := json.Marshal(orchestration)
	if err != nil {
		return fmt.Errorf("error marshalling orchestration: %w", err)
	}

	// Revision 0 only succeeds if the orchestration does not exist yet
	revision, err := o.Client.Update(ctx, orchestration.ID, serializedOrchestration, 0)
	if err != nil {
		if !isRevisionConflict(err) {
			return fmt.Errorf("error storing orchestration: %w", err)
		}
		// The orchestration already exists. Resume it if it was stored but its start did not complete.
		existing, existingRevision, err := ReadOrchestration(ctx, orchestration.ID, o.Client)
		if err != nil {
			return types.NewRecoverableWrappedError(err, "error reading existing orchestration %s", orchestration.ID)
		}
		if existing.State != api.OrchestrationStateInitialized {
			return nil
		}
		if o.monitor != nil {
			o.monitor.Infof("Orchestration %s was stored but not started, starting it", orchestration.ID)
		}
		span.AddEvent("Resuming orchestration that was not started")
		return o.start(ctx, existing, existingRevision)
	}

	initOrchMetrics()
	orchStartedCounter.Add(ctx, 1)
	return o.start(ctx, *orchestration, revision)
}

// start publishes the messages for the orchestration's first activities and then marks the orchestration running.
func (o *NatsOrchestrator) start(ctx context.Context, orchestration api.Orchestration, revision uint64) error {
	activities := getInitialActivities(&orchestration)
	if err := EnqueueActivityMessages(ctx, orchestration.ID, activities, o.Client); err != nil {
		return types.NewRecoverableWrappedError(err, "error enqueueing activities of orchestration %s", orchestration.ID)
	}
	trace.SpanFromContext(ctx).AddEvent("Activity messages enqueued")

	// Activities may already have updated the orchestration, or even finished it, so only move it out of initialized
	if _, _, err := UpdateOrchestration(ctx, orchestration, revision, o.Client, func(o *api.Orchestration) {
		if o.State == api.OrchestrationStateInitialized {
			o.SetState(api.OrchestrationStateRunning)
		}
	}); err != nil {
		return types.NewRecoverableWrappedError(err, "error marking orchestration %s as running", orchestration.ID)
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
