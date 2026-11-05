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
	"errors"
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

// DefaultActivityTimeout is how long an activity processor may run before its context is canceled. Activities that wait
// for something to happen should reschedule themselves rather than block, so the limit only stops activities that
// hang, for example on a downstream that does not answer.
const DefaultActivityTimeout = 5 * time.Minute

type NatsActivityExecutor struct {
	Client            natsclient.MsgClient
	StreamName        string
	ActivityType      string
	ActivityProcessor api.ActivityProcessor
	Monitor           system.LogMonitor
	// MaxActivityRetries is how often an activity may fail with a retriable error before the orchestration is marked
	// errored. It defaults to natsclient.DefaultMaxDeliver.
	MaxActivityRetries int
	// ActivityTimeout bounds how long the activity processor may run. It defaults to DefaultActivityTimeout.
	ActivityTimeout time.Duration
	// Loop drives message fetching. Execute sets Loop.Recreate if it is not already set.
	Loop natsclient.FetchLoop
}

func (e *NatsActivityExecutor) maxActivityRetries() int {
	if e.MaxActivityRetries <= 0 {
		return natsclient.DefaultMaxDeliver
	}
	return e.MaxActivityRetries
}

func (e *NatsActivityExecutor) activityTimeout() time.Duration {
	if e.ActivityTimeout <= 0 {
		return DefaultActivityTimeout
	}
	return e.ActivityTimeout
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

	if e.Loop.Recreate == nil {
		e.Loop.Recreate = func(ctx context.Context) (jetstream.Consumer, error) {
			stream, err := e.Client.Stream(ctx, e.StreamName)
			if err != nil {
				return nil, fmt.Errorf("error opening stream: %w", err)
			}
			return natsclient.SetupActivityConsumer(ctx, stream, e.ActivityType)
		}
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
// It runs continuously until the provided context is canceled, retrying fetch errors rather than exiting on them.
// Returns the context's error once it is canceled.
func (e *NatsActivityExecutor) processLoop(ctx context.Context, consumer jetstream.Consumer) error {
	return e.Loop.Run(ctx, consumer, e.Monitor, func(ctx context.Context, message jetstream.Msg) {
		if err := e.processMessage(ctx, message); err != nil {
			e.Monitor.Warnf("Error processing message: %v", err)
		}
	})
}

// processMessage processes a single message from the JetStream consumer by delegating to its ActivityProcessor. When
// processing is complete, the orchestration state is updated, messages for the next activities are enqueued if the
// orchestration can proceed, and the original message is acknowledged.
//
// Returns an error if message processing fails.
func (e *NatsActivityExecutor) processMessage(ctx context.Context, message jetstream.Msg) (err error) {
	defer natsclient.TermOnPanic(message, e.Monitor, &err)

	var oMessage api.ActivityMessage
	if err := json.Unmarshal(message.Data(), &oMessage); err != nil {
		ackErr := natsclient.AckMessage(message)
		if ackErr != nil {
			e.Monitor.Warnf("Failed to ACK message: %v", ackErr)
		}
		return fmt.Errorf("failed to unmarshal orchestration message: %w", err)
	}

	finalDelivery := natsclient.IsFinalDelivery(message, natsclient.ActivityMaxDeliver)

	orchestration, revision, err := ReadOrchestration(ctx, oMessage.OrchestrationID, e.Client)
	if err != nil {
		if finalDelivery {
			// Last chance: without the orchestration it cannot be marked errored, so drop the message loudly
			e.Monitor.Severef("Giving up on activity message %s for orchestration %s after %d deliveries: %v",
				oMessage.Activity.ID, oMessage.OrchestrationID, natsclient.ActivityMaxDeliver, err)
			if termErr := message.Term(); termErr != nil {
				err = errors.Join(err, fmt.Errorf("failed to TERM message: %w", termErr))
			}
		}
		return fmt.Errorf("failed to read orchestration data: %w", err)
	}

	if orchestration.ProcessingData == nil {
		orchestration.ProcessingData = make(map[string]any)
	}

	// attempt to restore trace ID from stored orchestration
	if traceParent := orchestration.EngineState.TraceContext; traceParent != "" {
		e.Monitor.Infof("Telemetry: continue trace from Orchestration's traceparent: %s", traceParent)
		carrier := propagation.MapCarrier{"traceparent": traceParent}
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
		orchestration.EngineState.TraceContext = "" // Clean up trace context after restoring it
	} else {
		e.Monitor.Debugf("Telemetry: continue trace from NATS message")
		// Extract trace context from message headers
		carrier := &natsHeaderCarrier{headers: message.Headers()}
		ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	}

	// Track attempt number for this activity — persisted across reschedules so all executions can be correlated
	if orchestration.EngineState.Attempts == nil {
		orchestration.EngineState.Attempts = make(map[string]int)
	}
	attempt := orchestration.EngineState.Attempts[oMessage.Activity.ID] + 1
	orchestration.EngineState.Attempts[oMessage.Activity.ID] = attempt

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

	if finalDelivery {
		// Safety net: the message has been redelivered so often that something is stuck, for example an activity that
		// reschedules forever or a completion that keeps failing. Stop here and mark the orchestration errored.
		e.Monitor.Severef("Giving up on activity %s of orchestration %s after %d deliveries",
			oMessage.Activity.ID, oMessage.OrchestrationID, natsclient.ActivityMaxDeliver)
		return e.handleFatalError(activityContext, orchestration, revision,
			fmt.Errorf("activity %s was delivered %d times without completing", oMessage.Activity.ID, natsclient.ActivityMaxDeliver), message)
	}

	e.Monitor.Debugf("Received activity message %s for orchestration %s (attempt %d)", oMessage.Activity.ID, oMessage.OrchestrationID, attempt)

	initActivityMetrics()
	activityAttrs := metric.WithAttributes(
		attribute.String("activity.type", string(oMessage.Activity.Type)),
		attribute.Int("activity.attempt", attempt),
	)
	activityStartedCounter.Add(ctx, 1, activityAttrs)
	activityStart := time.Now()

	result := e.runActivity(ctx, orchestration, oMessage.Activity)

	activityDurationHistogram.Record(ctx, float64(time.Since(activityStart).Milliseconds()),
		metric.WithAttributes(
			attribute.String("activity.type", string(oMessage.Activity.Type)),
			attribute.String("result", result.Result.String()),
			attribute.Int("activity.attempt", attempt),
		))

	switch result.Result {
	case api.ActivityResultRetryError:
		return e.handleRetryError(activityContext, orchestration, revision, message, oMessage.Activity.ID, result)

	case api.ActivityResultFatalError:
		return e.handleFatalError(activityContext, orchestration, revision, result.Error, message)

	case api.ActivityResultWait:
		e.persistState(activityContext, orchestration, revision)
		return natsclient.AckMessage(message)

	case api.ActivityResultSchedule:
		// IMPORTANT: Must persist state BEFORE rescheduling
		// This ensures processing data is saved for the next invocation

		// Inject trace context into the engine state so that subsequent executions are under the same trace-id
		m := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, m)
		orchestration.EngineState.TraceContext = m["traceparent"]

		e.persistState(activityContext, orchestration, revision)
		if err := message.NakWithDelay(result.WaitOnReschedule); err != nil {
			return fmt.Errorf("failed to reschedule schedule activity %s: %w", oMessage.OrchestrationID, err)
		}
		return nil
	}

	return e.processOnActivityCompletion(activityContext, orchestration, revision, message, oMessage)
}

// runActivity runs the activity processor with a deadline of ActivityTimeout. Only the processor gets the deadline: the
// executor's own updates afterwards use the caller's context, so that the outcome is still recorded after an activity
// ran out of time. An activity that fails because it ran out of time is retried rather than failed, since agents
// commonly report any error from a downstream as fatal, and a slow downstream must not fail the orchestration for good.
func (e *NatsActivityExecutor) runActivity(ctx context.Context, orchestration api.Orchestration, activity api.Activity) api.ActivityResult {
	timeout := e.activityTimeout()
	activityCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// The context shares the orchestration's data maps, so values set by the processor are seen by the executor
	activityContext := api.NewActivityContext(activityCtx, orchestration.ID, activity, orchestration.ProcessingData, orchestration.OutputData)

	var result api.ActivityResult
	switch activityContext.Discriminator() {
	case api.DeployDiscriminator:
		result = e.ActivityProcessor.ProcessDeploy(activityContext)
	case api.DisposeDiscriminator:
		result = e.ActivityProcessor.ProcessDispose(activityContext)
	default:
		result = e.ActivityProcessor.Process(activityContext)
	}

	if result.Result == api.ActivityResultFatalError && errors.Is(activityCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
		e.Monitor.Warnf("Activity %s of orchestration %s did not finish within %s, retrying it", activity.ID, orchestration.ID, timeout)
		resultErr := result.Error
		if resultErr == nil {
			resultErr = context.DeadlineExceeded
		}
		return api.ActivityResult{
			Result: api.ActivityResultRetryError,
			Error:  fmt.Errorf("activity %s did not finish within %s: %w", activity.ID, timeout, resultErr),
		}
	}
	return result
}

func (e *NatsActivityExecutor) persistState(activityContext api.ActivityContext, orchestration api.Orchestration, revision uint64) {
	engineState := orchestration.EngineState
	if _, _, err := UpdateOrchestration(activityContext.Context(), orchestration, revision, e.Client, func(o *api.Orchestration) {
		for key, value := range activityContext.Values() {
			o.ProcessingData[key] = value
		}
		for key, value := range activityContext.OutputValues() {
			o.OutputData[key] = value
		}
		o.EngineState.Merge(engineState)
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
	engineState := orchestration.EngineState
	orchestration, revision, err := UpdateOrchestration(activityContext.Context(), orchestration, revision, e.Client, func(o *api.Orchestration) {
		for key, value := range activityContext.Values() {
			o.ProcessingData[key] = value
		}

		for key, value := range activityContext.OutputValues() {
			o.OutputData[key] = value
		}
		o.EngineState.Merge(engineState)
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
		ID:                    uuid.New().String(),
		ManifestID:            orchestration.ID,
		CorrelationID:         orchestration.CorrelationID,
		Success:               orchestration.State != api.OrchestrationStateErrored,
		OrchestrationType:     orchestration.OrchestrationType,
		Properties:            orchestration.OutputData,
		CompensatedManifestID: orchestration.CompensatedOrchestrationID,
	}
	ser, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal orchestration response: %w", err)
	}
	_, err = e.Client.Publish(activityContext.Context(), natsclient.CFMOrchestrationResponseSubject, ser)
	return err
}

// handleRetryError handles retriable errors by persisting the orchestration state and redelivering the message after a
// delay: the activity's WaitOnReschedule if it set one, otherwise a backoff that grows with each retry. Once the
// activity has failed MaxActivityRetries times, the error is handled as fatal instead, so the orchestration is marked
// errored rather than retried forever.
func (e *NatsActivityExecutor) handleRetryError(
	activityContext api.ActivityContext,
	orchestration api.Orchestration,
	revision uint64,
	message jetstream.Msg,
	activityID string,
	result api.ActivityResult) error {

	resultErr := result.Error
	if resultErr == nil {
		resultErr = errors.New("activity requested a retry")
	}

	if orchestration.EngineState.Retries == nil {
		orchestration.EngineState.Retries = make(map[string]int)
	}
	retries := orchestration.EngineState.Retries[activityID] + 1
	orchestration.EngineState.Retries[activityID] = retries

	if retries >= e.maxActivityRetries() {
		e.Monitor.Severef("Giving up on activity %s of orchestration %s after %d failed attempts: %v",
			activityID, orchestration.ID, retries, resultErr)
		return e.handleFatalError(activityContext, orchestration, revision,
			fmt.Errorf("activity %s failed %d times: %w", activityID, retries, resultErr), message)
	}

	delay := result.WaitOnReschedule
	if delay <= 0 {
		delay = natsclient.RetryDelay(retries)
	}

	e.persistState(activityContext, orchestration, revision)
	if err := message.NakWithDelay(delay); err != nil {
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
	engineState := orchestration.EngineState
	if _, _, err := UpdateOrchestration(activityContext.Context(), orchestration, revision, e.Client, func(o *api.Orchestration) {
		if o.ProcessingData == nil {
			o.ProcessingData = make(map[string]any)
		}
		for key, value := range activityContext.Values() {
			o.ProcessingData[key] = value
		}
		if o.OutputData == nil {
			o.OutputData = make(map[string]any)
		}
		for key, value := range activityContext.OutputValues() {
			o.OutputData[key] = value
		}
		o.SetState(api.OrchestrationStateErrored)
		o.EngineState.Merge(engineState)
		o.EngineState.Error = resultErr.Error()
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
