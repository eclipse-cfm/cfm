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

package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serve(t *testing.T, handler http.HandlerFunc) (int, response) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	var body response
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&body))
	return recorder.Code, body
}

func ok(context.Context) error { return nil }

func TestRegistry_NoChecksIsUp(t *testing.T) {
	r := NewRegistry(nil)

	code, body := serve(t, r.LivenessHandler())
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, statusUp, body.Status)

	code, _ = serve(t, r.ReadinessHandler())
	assert.Equal(t, http.StatusOK, code)
}

// warnMonitor records the messages logged at warning level.
type warnMonitor struct {
	system.NoopMonitor
	warnings []string
}

func (m *warnMonitor) Warnf(message string, args ...any) {
	m.warnings = append(m.warnings, fmt.Sprintf(message, args...))
}

func TestRegistry_ReadinessFailureDoesNotFailLiveness(t *testing.T) {
	monitor := &warnMonitor{}
	r := NewRegistry(monitor)
	r.AddLiveness("loop", ok)
	r.AddReadiness("nats", func(context.Context) error { return errors.New("NATS connection is RECONNECTING") })

	code, body := serve(t, r.LivenessHandler())
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, map[string]string{"loop": statusUp}, body.Checks)

	code, body = serve(t, r.ReadinessHandler())
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, statusDown, body.Status)
	assert.Equal(t, statusUp, body.Checks["loop"])
	assert.Equal(t, statusDown, body.Checks["nats"], "the reason must not be served by the unauthenticated endpoint")
	assert.Equal(t, []string{"Health check nats failed: NATS connection is RECONNECTING"}, monitor.warnings)
}

func TestRegistry_LivenessFailureFailsReadiness(t *testing.T) {
	r := NewRegistry(nil)
	r.AddLiveness("loop", func(context.Context) error { return errors.New("stuck") })

	code, _ := serve(t, r.LivenessHandler())
	assert.Equal(t, http.StatusServiceUnavailable, code)

	code, _ = serve(t, r.ReadinessHandler())
	assert.Equal(t, http.StatusServiceUnavailable, code)
}

type fakeLoop struct {
	maxAges []time.Duration
	err     error
}

func (l *fakeLoop) Check(maxAge time.Duration) error {
	l.maxAges = append(l.maxAges, maxAge)
	return l.err
}

func TestRegistry_AddConsumerLoop(t *testing.T) {
	r := NewRegistry(nil)
	loop := &fakeLoop{}
	r.AddConsumerLoop("consumer", loop)

	serve(t, r.LivenessHandler())
	serve(t, r.ReadinessHandler())

	assert.Equal(t, []time.Duration{ConsumerLivenessMaxAge, ConsumerReadinessMaxAge}, loop.maxAges,
		"readiness must use the stricter bound in place of the liveness one")
}

type fakePinger struct{ err error }

func (p fakePinger) PingContext(context.Context) error { return p.err }

func TestDatabaseCheck(t *testing.T) {
	require.NoError(t, DatabaseCheck(fakePinger{})(t.Context()))
	require.Error(t, DatabaseCheck(fakePinger{err: errors.New("connection refused")})(t.Context()))
}

func TestNatsConnectionCheck_NilConnection(t *testing.T) {
	require.Error(t, NatsConnectionCheck(nil)(t.Context()))
}

func TestRegisterOptional(t *testing.T) {
	services := system.NewServiceRegistry()
	called := false
	RegisterOptional(services, func(*Registry) { called = true })
	assert.False(t, called, "nothing is registered without a health registry")

	registry := NewRegistry(nil)
	services.Register(RegistryKey, registry)
	RegisterOptional(services, func(r *Registry) {
		called = true
		assert.Same(t, registry, r)
	})
	assert.True(t, called)
}
