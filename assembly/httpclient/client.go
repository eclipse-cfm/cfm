// Copyright (c) 2025 Metaform Systems, Inc
//
// This program and the accompanying materials are made available under the
// terms of the Apache License, Version 2.0 which is available at
// https://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0
//
// Contributors:
//
//	Metaform Systems, Inc. - initial API and implementation

package httpclient

import (
	"context"
	"net/http"
	"time"

	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/hashicorp/go-retryablehttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

const (
	ConfigKeyRetryMax     string = "httpclient.retrymax"
	DefaultRetryMax       int    = 5
	ConfigKeyRetryWaitMin string = "httpclient.retrywaitmin"
	DefaultRetryWaitMin   int    = 1
	ConfigKeyRetryWaitMax string = "httpclient.retrywaitmax"
	DefaultRetryWaitMax   int    = 5
	// ConfigKeyTimeout is the time in seconds a single request attempt may take, including reading the response
	// body. An attempt that times out is retried like any other connection error.
	ConfigKeyTimeout string = "httpclient.timeout"
	DefaultTimeout   int    = 30
)

// clientConfig holds the settings of the shared HTTP client.
type clientConfig struct {
	retryMax     int
	retryWaitMin time.Duration
	retryWaitMax time.Duration
	timeout      time.Duration
}

type HttpClientServiceAssembly struct {
	system.DefaultServiceAssembly
}

func (h HttpClientServiceAssembly) Name() string {
	return "HttpClient"
}

func (h HttpClientServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{serviceapi.HttpClientKey}
}

func (h HttpClientServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{}
}

func (h HttpClientServiceAssembly) Init(context *system.InitContext) error {
	config := clientConfig{
		retryMax:     context.GetConfigIntOrDefault(ConfigKeyRetryMax, DefaultRetryMax),
		retryWaitMin: time.Duration(context.GetConfigIntOrDefault(ConfigKeyRetryWaitMin, DefaultRetryWaitMin)) * time.Second,
		retryWaitMax: time.Duration(context.GetConfigIntOrDefault(ConfigKeyRetryWaitMax, DefaultRetryWaitMax)) * time.Second,
		timeout:      time.Duration(context.GetConfigIntOrDefault(ConfigKeyTimeout, DefaultTimeout)) * time.Second,
	}
	standardClient := newClient(config, &routingLogger{monitor: context.LogMonitor})
	context.Registry.Register(serviceapi.HttpClientKey, *standardClient)
	return nil
}

// newClient creates an HTTP client that retries failed requests. Each attempt is bounded by the configured timeout, so
// a server that accepts the connection but never answers cannot block the caller indefinitely.
func newClient(config clientConfig, logger retryablehttp.LeveledLogger) *http.Client {
	retryClient := retryablehttp.NewClient()

	retryClient.RetryMax = config.retryMax
	retryClient.RetryWaitMin = config.retryWaitMin
	retryClient.RetryWaitMax = config.retryWaitMax
	retryClient.CheckRetry = customCheckRetry
	retryClient.Logger = logger
	if config.timeout > 0 {
		retryClient.HTTPClient.Timeout = config.timeout
	}
	standardClient := retryClient.StandardClient()

	// Wrap the client with OpenTelemetry instrumentation
	standardClient.Transport = otelhttp.NewTransport(standardClient.Transport,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}))

	return standardClient
}

// customCheckRetry determines whether a request should be retried
// It will not retry on 4xx client errors (bad requests)
func customCheckRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// Don't retry if the context is canceled or timed out
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// Don't retry on 4xx client errors (bad requests)
	if resp != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return false, nil
	}

	// Use the default retry logic for other cases
	return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
}

// routingLogger routes HTTP client log messages to the system.LogMonitor instance.
type routingLogger struct {
	monitor system.LogMonitor
}

func (r routingLogger) Error(msg string, keysAndValues ...any) {
	r.monitor.Severef(msg, keysAndValues...)
}

func (r routingLogger) Info(msg string, keysAndValues ...any) {
	r.monitor.Infof(msg, keysAndValues...)
}

func (r routingLogger) Debug(msg string, keysAndValues ...any) {
	r.monitor.Debugf(msg, keysAndValues...)
}

func (r routingLogger) Warn(msg string, keysAndValues ...any) {
	r.monitor.Infof(msg, keysAndValues...)
}
