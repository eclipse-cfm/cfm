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

package natsorchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
)

var (
	activityMetricsOnce       sync.Once
	activityStartedCounter    metric.Int64Counter
	activityDurationHistogram metric.Float64Histogram
)

func initActivityMetrics() {
	activityMetricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter("cfm.pmanager.orchestrator")
		activityStartedCounter, _ = meter.Int64Counter("cfm.activity.started.total")
		activityDurationHistogram, _ = meter.Float64Histogram("cfm.activity.duration",
			metric.WithUnit("ms"))
	})
}

type NatsActivityExecutor struct {
	Client            natsclient.MsgClient
	StreamName        string
	ActivityType      string
	ActivityProcessor api.ActivityProcessor
	Monitor           system.LogMonitor
}

// Execute starts a goroutine to process messages from the activity queue.
func (e *NatsActivityExecutor) Execute(ctx context.Context) error {
	stream, err := e.Client.Stream(ctx, e.StreamName)
	if err != nil {
		return fmt.Errorf("error opening stream: %w", err)
	}

	consumerName := strings.ReplaceAll(e.ActivityType, ".", "-")
	consumer, err := stream.Consumer(ctx, consumerName)
	if err != nil {
		return fmt.Errorf("error connecting to consumer %s: %w", consumerName, err)
	}

	go func() {
		err := e.processLoop(ctx, consumer)
		if err != nil {
			e.Monitor.Warnf("Error processing message: %v", err)
		}
	}()
	return nil
}

// processLoop handles the main loop for consuming and processing messages from a JetStream consumer.
// It runs continuously until the provided context is canceled or an error occurs.
// Returns an error if message fetching or processing fails.
func (e *NatsActivityExecutor) processLoop(ctx context.Context, consumer jetstream.Consumer) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			messageBatch, err := consumer.Fetch(1, jetstream.FetchMaxWait(time.Second))
			if err != nil {
				return err
			}

			for message := range messageBatch.Messages() {
				if err = e.processMessage(ctx, message); err != nil {
					e.Monitor.Warnf("Error processing message: %v", err)
				}
			}
		}
	}
}

// processMessage processes a single message from the JetStream consumer by delegating to its ActivityProcessor. When
// processing is complete, the orchestration state is updated, messages for the next activities are enqueued if the
// orchestration can proceed, and the original message is acknowledged.
//
// Returns an error if message processing fails.
func (e *NatsActivityExecutor) processMessage(ctx context.Context, message jetstream.Msg) error {

	var oMessage api.ActivityMessage
	if err := json.Unmarshal(message.Data(), &oMessage); err != nil {
		ackErr := natsclient.AckMessage(message)
		if ackErr != nil {
			e.Monitor.Warnf("Failed to ACK message: %v", ackErr)
		}
		return fmt.Errorf("failed to unmarshal orchestration message: %w", err)
	}

	orchestration, revision, err := ReadOrchestration(ctx, oMessage.OrchestrationID, e.Client)
	if err != nil {
		return fmt.Errorf("failed to read orchestration data: %w", err)
	}

	// attempt to restore trace ID from stored orchestration
	if orchestration.ProcessingData["trace_context"] != nil {
		traceParent := orchestration.ProcessingData["trace_context"].(string)
		e.Monitor.Infof("Telemetry: continue trace from Orchestration's traceparent: %s", traceParent)
		carrier := propagation.MapCarrier{"traceparent": traceParent}
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		delete(orchestration.ProcessingData, "trace_context") // Clean up trace context from processing data after restoring it
	} else {
		e.Monitor.Debugf("Telemetry: continue trace from NATS message")
		// Extract trace context from message headers
		carrier := &natsHeaderCarrier{headers: message.Headers()}
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	}

	// Track attempt number for this activity — persisted across reschedules so all executions can be correlated
	attemptKey := "activity." + oMessage.Activity.ID + ".attempt"
	attempt := 1
	if raw, ok := orchestration.ProcessingData[attemptKey]; ok {
		if v, ok := raw.(float64); ok { // JSON unmarshal stores numbers as float64
			attempt = int(v) + 1
		}
	}
	orchestration.ProcessingData[attemptKey] = attempt

	// Start span with extracted context
	ctx, span := otel.GetTracerProvider().Tracer("cfm.pmanager.orchestrator").Start(ctx, "activity.process_message")
	defer span.End()
	span.SetAttributes(
		attribute.String("activity.type", string(oMessage.Activity.Type)),
		attribute.Int("activity.attempt", attempt),
	)

	activityContext := api.NewActivityContext(
		ctx,
		orchestration.ID,
		oMessage.Activity,
		orchestration.ProcessingData,
		orchestration.OutputData)

	e.Monitor.Debugf("Received activity message %s for orchestration %s (attempt %d)", oMessage.Activity.ID, oMessage.OrchestrationID, attempt)

	initActivityMetrics()
	activityAttrs := metric.WithAttributes(
		attribute.String("activity.type", string(oMessage.Activity.Type)),
		attribute.Int("activity.attempt", attempt),
	)
	activityStartedCounter.Add(ctx, 1, activityAttrs)
	activityStart := time.Now()

	var result api.ActivityResult
	if activityContext.Discriminator() == api.DeployDiscriminator {
		result = e.ActivityProcessor.ProcessDeploy(activityContext)
	} else if activityContext.Discriminator() == api.DisposeDiscriminator {
		result = e.ActivityProcessor.ProcessDispose(activityContext)
	} else {
		result = e.ActivityProcessor.Process(activityContext)
	}

	activityDurationHistogram.Record(ctx, float64(time.Since(activityStart).Milliseconds()),
		metric.WithAttributes(
			attribute.String("activity.type", string(oMessage.Activity.Type)),
			attribute.String("result", result.Result.String()),
			attribute.Int("activity.attempt", attempt),
		))

	switch result.Result {
	case api.ActivityResultRetryError:
		return e.handleRetryError(activityContext, orchestration, revision, message, result.Error)

	case api.ActivityResultFatalError:
		return e.handleFatalError(activityContext, orchestration, revision, result.Error, message)

	case api.ActivityResultWait:
		e.persistState(activityContext, orchestration, revision)
		return natsclient.AckMessage(message)

	case api.ActivityResultSchedule:
		// IMPORTANT: Must persist state BEFORE rescheduling
		// This ensures processing data is saved for the next invocation

		// Inject trace context into processing data so that subsequent executions are under the same trace-id
		m := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, m)
		if m["traceparent"] != "" {
			orchestration.ProcessingData["trace_context"] = m["traceparent"]
		}

		e.persistState(activityContext, orchestration, revision)
		if err := message.NakWithDelay(result.WaitOnReschedule); err != nil {
			return fmt.Errorf("failed to reschedule schedule activity %s: %w", oMessage.OrchestrationID, err)
		}
		return nil
	}

	return e.processOnActivityCompletion(activityContext, orchestration, revision, message, oMessage)
}

func (e *NatsActivityExecutor) persistState(activityContext api.ActivityContext, orchestration api.Orchestration, revision uint64) {
	if _, _, err := UpdateOrchestration(activityContext.Context(), orchestration, revision, e.Client, func(o *api.Orchestration) {
		for key, value := range activityContext.Values() {
			o.ProcessingData[key] = value
		}
		for key, value := range activityContext.OutputValues() {
			o.OutputData[key] = value
		}
	}); err != nil {
		e.Monitor.Warnf("Failed to persist orchestration state for %s: %v", orchestration.ID, err)
	}
}

func (e *NatsActivityExecutor) processOnActivityCompletion(
	activityContext api.ActivityContext,
	orchestration api.Orchestration,
	revision uint64,
	message jetstream.Msg,
	oMessage api.ActivityMessage) error {

	// The orchestration state must be saved and re-read to determine if activities completed after the last read and the orchestration is complete.
	orchestration, revision, err := UpdateOrchestration(activityContext.Context(), orchestration, revision, e.Client, func(o *api.Orchestration) {
		for key, value := range activityContext.Values() {
			o.ProcessingData[key] = value
		}

		for key, value := range activityContext.OutputValues() {
			o.OutputData[key] = value
		}
		o.Completed[oMessage.Activity.ID] = struct{}{} // Mark current activity as completed
	})
	if err != nil {
		err = natsclient.NakError(message, err)
		return err
	}

	// Return if orchestration is in the error state since processing should stop
	if orchestration.State == api.OrchestrationStateErrored {
		return natsclient.AckMessage(message)
	}

	// Check if all parallel activities have completed and the orchestration can continue to the next step
	canProceed, err := orchestration.CanProceedToNextStep(oMessage.Activity.ID)
	if err != nil {
		err = natsclient.NakError(message, err)
		return fmt.Errorf("failed to proceed with orchestration %s: %w", oMessage.OrchestrationID, err)
	}

	if !canProceed {
		// Waiting for parallel activities to complete
		return natsclient.AckMessage(message)
	}

	next := orchestration.GetNextStepActivities(oMessage.Activity.ID)
	if len(next) == 0 {
		// No more steps, mark as completed
		return e.handleOrchestrationCompletion(activityContext, orchestration, revision, message)
	}

	// Enqueue next activities
	if err := EnqueueActivityMessages(activityContext.Context(), orchestration.ID, next, e.Client); err != nil {
		// Failed redeliver the message
		err = natsclient.NakError(message, err)
		return fmt.Errorf("failed to enqueue next orchestration activities %s: %w", oMessage.OrchestrationID, err)
	}

	return natsclient.AckMessage(message)
}

func (e *NatsActivityExecutor) handleOrchestrationCompletion(
	activityContext api.ActivityContext,
	orchestration api.Orchestration,
	revision uint64,
	message jetstream.Msg) error {
	// Mark as completed
	_, _, err := UpdateOrchestration(activityContext.Context(), orchestration, revision, e.Client, func(o *api.Orchestration) {
		o.SetState(api.OrchestrationStateCompleted)
	})
	if err != nil {
		// Error marking, redeliver the message
		err = natsclient.NakError(message, err)
		return fmt.Errorf("failed to mark orchestration %s as completed: %v", orchestration.ID, err)
	}

	initOrchMetrics()
	orchCompletedCounter.Add(activityContext.Context(), 1, metric.WithAttributes(attribute.String("result", "success")))
	orchDurationHistogram.Record(activityContext.Context(), float64(time.Since(orchestration.CreatedTimestamp).Milliseconds()),
		metric.WithAttributes(attribute.String("result", "success")))

	err = e.publishResponse(activityContext, orchestration)
	if err != nil {
		return err
	}

	return natsclient.AckMessage(message)
}

func (e *NatsActivityExecutor) publishResponse(activityContext api.ActivityContext, orchestration api.Orchestration) error {
	response := &model.OrchestrationResponse{
		ID:                uuid.New().String(),
		ManifestID:        orchestration.ID,
		CorrelationID:     orchestration.CorrelationID,
		Success:           orchestration.State != api.OrchestrationStateErrored,
		OrchestrationType: orchestration.OrchestrationType,
		Properties:        orchestration.OutputData,
	}
	ser, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal orchestration response: %w", err)
	}
	_, err = e.Client.Publish(activityContext.Context(), natsclient.CFMOrchestrationResponseSubject, ser)
	return err
}

// handleRetryError handles retriable errors by persisting the orchestration state and re-delivering the message using a Nak.
func (e *NatsActivityExecutor) handleRetryError(
	activityContext api.ActivityContext,
	orchestration api.Orchestration,
	revision uint64,
	message jetstream.Msg,
	resultErr error) error {

	e.persistState(activityContext, orchestration, revision)
	// Nak to redeliver the message
	if err := message.Nak(); err != nil {
		return fmt.Errorf("retriable failure when executing activity message and NAK response %s (errors: %w, %v)",
			orchestration.ID, resultErr, err)
	}
	return fmt.Errorf("retriable failure when executing activity %s: %w", orchestration.ID, resultErr)
}

// handleFatalError handles unrecoverable errors by updating the orchestration state to "Errored" and acknowledging the message.
// It ensures acknowledgments are sent to avoid message re-delivery, even if the state update fails.
// Returns an error with specific details about the fatal failure.
func (e *NatsActivityExecutor) handleFatalError(
	activityContext api.ActivityContext,
	orchestration api.Orchestration,
	revision uint64,
	resultErr error,
	message jetstream.Msg) error {
	// Update the orchestration before acking back. If the update fails, just log it to ensure the ack is sent to avoid message re-delivery
	if _, _, err := UpdateOrchestration(activityContext.Context(), orchestration, revision, e.Client, func(o *api.Orchestration) {
		for key, value := range activityContext.Values() {
			orchestration.ProcessingData[key] = value
		}
		for key, value := range activityContext.OutputValues() {
			orchestration.OutputData[key] = value
		}
		o.SetState(api.OrchestrationStateErrored)
		if o.ProcessingData == nil {
			o.ProcessingData = make(map[string]any)
		}
		o.ProcessingData["error"] = resultErr.Error()
		orchestration = *o
	}); err != nil {
		e.Monitor.Warnf("Failed to mark orchestration %s as fatal: %v", orchestration.ID, err)
	}

	initOrchMetrics()
	orchCompletedCounter.Add(activityContext.Context(), 1, metric.WithAttributes(attribute.String("result", "fatal")))
	orchDurationHistogram.Record(activityContext.Context(), float64(time.Since(orchestration.CreatedTimestamp).Milliseconds()),
		metric.WithAttributes(attribute.String("result", "fatal")))

	err := e.publishResponse(activityContext, orchestration)
	if err != nil {
		return err
	}

	if err := message.Ack(); err != nil {
		return fmt.Errorf("fatal failure while executing activity %s (errors: %w, %v)",
			orchestration.ID, resultErr, err)
	}
	return fmt.Errorf("fatal failure while executing activity %s: %w", orchestration.ID, resultErr)
}

// natsHeaderCarrier implements propagation.TextMapCarrier for NATS message headers
type natsHeaderCarrier struct {
	headers nats.Header
}

func (c *natsHeaderCarrier) Get(key string) string {
	return c.headers.Get(key)
}

func (c *natsHeaderCarrier) Set(key string, value string) {
	c.headers.Set(key, value)
}

func (c *natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.headers))
	for key := range c.headers {
		keys = append(keys, key)
	}
	return keys
}
