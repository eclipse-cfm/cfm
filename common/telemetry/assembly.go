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

package telemetry

import (
	"context"
	"fmt"

	"github.com/eclipse-cfm/cfm/common/system"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

const TracerProviderKey system.ServiceType = "telemetry:TracerProvider"

type TelemetryServiceAssembly struct {
	system.DefaultServiceAssembly
	ServiceName string
	tp          *sdktrace.TracerProvider
}

func NewTelemetryServiceAssembly(serviceName string) system.ServiceAssembly {
	return &TelemetryServiceAssembly{ServiceName: serviceName}
}

func (t *TelemetryServiceAssembly) Name() string {
	return "OpenTelemetry"
}

func (t *TelemetryServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{TracerProviderKey}
}

func (t *TelemetryServiceAssembly) Init(ctx *system.InitContext) error {

	spanCtx := context.Background()
	spanExporter, err := autoexport.NewSpanExporter(spanCtx)
	if err != nil {
		return err
	}

	res, err := resource.New(spanCtx,
		resource.WithFromEnv(),
		resource.WithAttributes(semconv.ServiceNameKey.String(t.ServiceName)),
	)
	if err != nil {
		return fmt.Errorf("failed to create telemetry resource: %w", err)
	}

	t.tp = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(t.tp)
	ctx.Registry.Register(TracerProviderKey, t.tp)
	return nil
}

func (t *TelemetryServiceAssembly) Shutdown() error {
	if t.tp != nil {
		return t.tp.Shutdown(context.Background())
	}
	return nil
}
