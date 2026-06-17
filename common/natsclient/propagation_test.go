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

package natsclient

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestNatsHeaderCarrier_PublishConsumeRoundTrip guards against a regression where
// the publish side injected via propagation.HeaderCarrier (an http.Header that
// canonicalizes keys to "Traceparent") while the consume side extracts via the
// case-sensitive nats.Header.Get, which only ever looks up the W3C lowercase
// "traceparent" key. That mismatch silently dropped the parent context, causing
// every consumer to start a brand-new root span (disconnected traces in Jaeger).
//
// Both publish and consume must use the same lowercase carrier so the span
// context survives the NATS hop.
func TestNatsHeaderCarrier_PublishConsumeRoundTrip(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	traceID, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	require.NoError(t, err)

	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	publishCtx := trace.ContextWithSpanContext(context.Background(), parent)

	// Inject exactly as the publisher does (see natsClientAdapter.Publish).
	headers := nats.Header{}
	otel.GetTextMapPropagator().Inject(publishCtx, &natsHeaderCarrier{headers: headers})

	// W3C mandates the lowercase "traceparent" key. The previous HeaderCarrier
	// stored it as "Traceparent", which nats.Header.Get could never read back.
	require.Contains(t, headers, "traceparent", "trace context must be written under the lowercase W3C key")
	require.NotContains(t, headers, "Traceparent", "header key must not be canonicalized for NATS")

	// Extract exactly as the consumer does (see RetriableMessageProcessor.ProcessMessage).
	consumeCtx := otel.GetTextMapPropagator().Extract(context.Background(), &natsHeaderCarrier{headers: headers})
	extracted := trace.SpanContextFromContext(consumeCtx)

	require.True(t, extracted.IsValid(), "extracted span context must be valid")
	assert.Equal(t, parent.TraceID(), extracted.TraceID(), "trace id must survive the NATS hop")
	assert.Equal(t, parent.SpanID(), extracted.SpanID(), "parent span id must survive the NATS hop")
	assert.True(t, extracted.IsRemote(), "extracted context must be marked remote")
}
