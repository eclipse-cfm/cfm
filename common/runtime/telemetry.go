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

package runtime

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	// Activate the OpenTelemetry Auto SDK. When the OpenTelemetry Go instrumentation
	// agent is present, it configures this SDK automatically via eBPF-based instrumentation.
	_ "go.opentelemetry.io/auto/sdk"
)

func init() {
	// Configure the global text map propagator for distributed tracing context propagation.
	// W3C TraceContext propagates trace/span IDs; W3C Baggage propagates key-value metadata.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}
