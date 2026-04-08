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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const TracerProviderKey system.ServiceType = "telemetry:TracerProvider"

type telemetryServiceAssembly struct {
	system.DefaultServiceAssembly
	serviceName string
	tp          *sdktrace.TracerProvider
}

func NewTelemetryServiceAssembly(serviceName string) system.ServiceAssembly {
	return &telemetryServiceAssembly{serviceName: serviceName}
}

func (t *telemetryServiceAssembly) Name() string {
	return "Telemetry"
}

func (t *telemetryServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{TracerProviderKey}
}

func (t *telemetryServiceAssembly) Init(ctx *system.InitContext) error {

	spanCtx := context.Background()
	spanExporter, err := autoexport.NewSpanExporter(spanCtx)
	if err != nil {
		return err
	}

	res, err := resource.New(spanCtx,
		resource.WithFromEnv(),
		resource.WithAttributes(attribute.String("service.name", t.serviceName)),
	)
	if err != nil {
		return fmt.Errorf("failed to create telemetry resource: %w", err)
	}

	t.tp = sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(spanExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(t.tp)
	ctx.Registry.Register(TracerProviderKey, t.tp)
	return nil
}

func (t *telemetryServiceAssembly) Shutdown() error {
	if t.tp != nil {
		return t.tp.Shutdown(context.Background())
	}
	return nil
}
