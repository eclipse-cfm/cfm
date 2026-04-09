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
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/eclipse-cfm/cfm/common/system"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	mode = "mode"
)

func LoadLogMonitor(name string, mode system.RuntimeMode) system.LogMonitor {
	var config zap.Config
	var options []zap.Option

	switch mode {
	case system.DebugMode, system.DevelopmentMode:
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.EncoderConfig.StacktraceKey = "stacktrace"
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	default:
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		// Disable stacktrace in production
		config.EncoderConfig.StacktraceKey = ""
	}

	config.DisableCaller = true

	// Add caller skip for accurate source locations
	options = append(options, zap.AddCallerSkip(1))

	logger, err := config.Build(options...)
	if err != nil {
		panic(fmt.Errorf("failed to initialize logger: %w", err))
	}

	return NewSugaredLogMonitor(logger.Named(name).Sugar())
}

var (
	loadModeOnce sync.Once
	runtimeMode  system.RuntimeMode
)

func LoadMode() system.RuntimeMode {
	// Guard against multiple calls when more than one runtime is loaded in the same process
	loadModeOnce.Do(func() {
		var modeFlag *string

		// Check if flag is already defined
		if flag.Lookup(mode) == nil {
			modeFlag = flag.String(mode, system.ProductionMode, "Runtime mode: development, production, or debug")
		} else {
			// Get the existing flag value
			modeValue := flag.Lookup(mode).Value.String()
			modeFlag = &modeValue
		}
		flag.Parse()

		parsedMode, err := system.ParseRuntimeMode(*modeFlag)
		if err != nil {
			panic(fmt.Errorf("error parsing runtime mode: %w", err))
		}
		runtimeMode = parsedMode
	})

	return runtimeMode
}

// SugaredLogMonitor implements LogMonitor by wrapping a zap.SugaredLogger
type SugaredLogMonitor struct {
	logger *zap.SugaredLogger
}

// NewSugaredLogMonitor creates a new LogMonitor that wraps a zap.SugaredLogger
func NewSugaredLogMonitor(logger *zap.SugaredLogger) system.LogMonitor {
	return &SugaredLogMonitor{logger: logger}
}

func (s *SugaredLogMonitor) Named(name string) system.LogMonitor {
	return &SugaredLogMonitor{logger: s.logger.Named(name)}
}

func (s *SugaredLogMonitor) Severef(message string, args ...any) {
	s.logger.Errorf(message, args...)
}

func (s *SugaredLogMonitor) Warnf(message string, args ...any) {
	s.logger.Warnf(message, args...)
}

func (s *SugaredLogMonitor) Infof(message string, args ...any) {
	s.logger.Infof(message, args...)
}

func (s *SugaredLogMonitor) Debugf(message string, args ...any) {
	s.logger.Debugf(message, args...)
}

func (s *SugaredLogMonitor) Severew(message string, keysValues ...any) {
	s.logger.Errorw(message, keysValues...)
}

func (s *SugaredLogMonitor) Warnw(message string, keysValues ...any) {
	s.logger.Warnw(message, keysValues...)
}

func (s *SugaredLogMonitor) Infow(message string, keysValues ...any) {
	s.logger.Infow(message, keysValues...)
}

func (s *SugaredLogMonitor) Debugw(message string, keysValues ...any) {
	s.logger.Debugw(message, keysValues...)
}

func (s *SugaredLogMonitor) Sync() error {
	return s.logger.Sync()
}

// AssembleAndLaunch assembles and launches the runtime with the given name and configuration.
// The runtime will be shutdown when the program is terminated.
func AssembleAndLaunch(assembler *system.ServiceAssembler, name string, monitor system.LogMonitor, shutdown <-chan struct{}) {
	err := assembler.Assemble()
	if err != nil {
		panic(fmt.Errorf("error assembling runtime: %w", err))
	}

	monitor.Infof("%s started", name)

	// wait for interrupt signal
	<-shutdown

	if err := assembler.Shutdown(); err != nil {
		monitor.Severew("Error attempting shutdown", "error", err)
	}

	monitor.Infof("%s shutdown", name)
}

// CreateSignalShutdownChan creates and returns a channel that signals when the application receives SIGINT or SIGTERM signals.
func CreateSignalShutdownChan() <-chan struct{} {
	shutdown := make(chan struct{})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		close(shutdown)
	}()

	return shutdown
}

// CheckRequiredParams validates that all keys have corresponding non-nil, non-empty values in the provided parameters.
// Returns an error if an odd number of parameters is given or if any key-value pairs have invalid values.
func CheckRequiredParams(params ...any) error {
	var errors []string

	if len(params)%2 != 0 {
		errors = append(errors, fmt.Sprintf("arguments must be even, got %d", len(params)))
	}

	for i := 0; i < len(params); i++ {
		if i%2 == 1 {
			// Even index (0, 2, 4, ...) - check if nil
			if params[i] == nil {
				errors = append(errors, fmt.Sprintf("%v not specified", params[i-1]))
			} else if str, ok := params[i].(string); ok && str == "" {
				errors = append(errors, fmt.Sprintf("%v is empty", params[i-1]))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("missing parameters: %s", strings.Join(errors, ", "))
	}

	return nil
}

func SetupTelemetry(serviceName string, shutdown <-chan struct{}) error {
	spanCtx := context.Background()
	spanExporter, err := autoexport.NewSpanExporter(spanCtx)
	if err != nil {
		return err
	}

	res, err := resource.New(spanCtx,
		resource.WithFromEnv(),
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return fmt.Errorf("failed to set up telemetry for service '%w':  %w", serviceName, err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp) // with this, just use otel.GetTracerProvider() to obtain it

	go func() {
		<-shutdown
		if err := tp.Shutdown(spanCtx); err != nil {
			// Log error but continue shutdown
			_ = err
		}
	}()

	return nil
}
