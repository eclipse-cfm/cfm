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

// Package health collects the liveness and readiness checks of a runtime and serves them over HTTP.
//
// Liveness checks answer whether the process should be restarted, for example because a consumer loop has stopped
// fetching messages. Readiness checks answer whether the process can currently do useful work, for example whether
// its NATS connection and database are reachable. Dependency outages should only fail readiness, since restarting
// the process does not fix them.
package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go"
)

const (
	RegistryKey system.ServiceType = "health:Registry"

	// LivenessPath and ReadinessPath are the unauthenticated endpoints infrastructure probes call.
	LivenessPath  = "/healthz"
	ReadinessPath = "/readyz"

	// ConsumerReadinessMaxAge and ConsumerLivenessMaxAge bound how long a consumer loop may go without completing a
	// fetch. A loop fetches at least once per second when healthy, so exceeding the readiness bound means it is
	// retrying errors; exceeding the liveness bound means it has not recovered and the process should be restarted.
	ConsumerReadinessMaxAge = 30 * time.Second
	ConsumerLivenessMaxAge  = 2 * time.Minute

	checkTimeout = 2 * time.Second
	statusUp     = "UP"
	statusDown   = "DOWN"
)

// Check reports nil if the checked component is healthy.
type Check func(ctx context.Context) error

// Registry holds the named liveness and readiness checks of a runtime. It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	liveness  map[string]Check
	readiness map[string]Check
	monitor   system.LogMonitor
}

// NewRegistry creates a registry that logs why a check failed to monitor, which may be nil.
func NewRegistry(monitor system.LogMonitor) *Registry {
	if monitor == nil {
		monitor = system.NoopMonitor{}
	}
	return &Registry{
		liveness:  make(map[string]Check),
		readiness: make(map[string]Check),
		monitor:   monitor,
	}
}

// AddLiveness registers a liveness check, replacing any check previously registered under the same name.
func (r *Registry) AddLiveness(name string, check Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.liveness[name] = check
}

// AddReadiness registers a readiness check, replacing any check previously registered under the same name.
func (r *Registry) AddReadiness(name string, check Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readiness[name] = check
}

// LivenessHandler serves the result of the liveness checks.
func (r *Registry) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.serve(w, req, r.snapshot(r.liveness))
	}
}

// ReadinessHandler serves the result of the readiness checks. A runtime is only ready if it is also alive, so the
// liveness checks are included.
func (r *Registry) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		checks := r.snapshot(r.liveness)
		for name, check := range r.snapshot(r.readiness) {
			checks[name] = check
		}
		r.serve(w, req, checks)
	}
}

func (r *Registry) snapshot(checks map[string]Check) map[string]Check {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Check, len(checks))
	for name, check := range checks {
		result[name] = check
	}
	return result
}

type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func (r *Registry) serve(w http.ResponseWriter, req *http.Request, checks map[string]Check) {
	ctx, cancel := context.WithTimeout(req.Context(), checkTimeout)
	defer cancel()

	names := make([]string, 0, len(checks))
	for name := range checks {
		names = append(names, name)
	}
	sort.Strings(names)

	result := response{Status: statusUp, Checks: make(map[string]string, len(checks))}
	for _, name := range names {
		if err := checks[name](ctx); err != nil {
			// The endpoints are unauthenticated, so the reason, which may name hosts or contain driver errors, is only
			// logged
			r.monitor.Warnf("Health check %s failed: %v", name, err)
			result.Status = statusDown
			result.Checks[name] = statusDown
		} else {
			result.Checks[name] = statusUp
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if result.Status == statusDown {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(result)
}

// NatsConnectionCheck reports whether a NATS connection is currently connected. A connection that is reconnecting
// is reported as unhealthy, so it should be registered as a readiness check.
func NatsConnectionCheck(conn *nats.Conn) Check {
	return func(context.Context) error {
		if conn == nil {
			return errors.New("no NATS connection")
		}
		if !conn.IsConnected() {
			return errors.New("NATS connection is " + conn.Status().String())
		}
		return nil
	}
}

// Pinger is implemented by *sql.DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// DatabaseCheck reports whether a database answers a ping.
func DatabaseCheck(db Pinger) Check {
	return func(ctx context.Context) error {
		return db.PingContext(ctx)
	}
}

// ConsumerLoop is implemented by natsclient.FetchLoop.
type ConsumerLoop interface {
	Check(maxAge time.Duration) error
}

// AddConsumerLoop registers the liveness and readiness checks of a consumer loop.
func (r *Registry) AddConsumerLoop(name string, loop ConsumerLoop) {
	r.AddLiveness(name, func(context.Context) error { return loop.Check(ConsumerLivenessMaxAge) })
	r.AddReadiness(name, func(context.Context) error { return loop.Check(ConsumerReadinessMaxAge) })
}

// Resolver is implemented by the service registry and the agent registries wrapping it.
type Resolver interface {
	ResolveOptional(serviceType system.ServiceType) (any, bool)
}

// RegisterOptional resolves the health registry from the service registry and passes it to register. It does nothing
// if the runtime has no health registry, so components can contribute checks without requiring one. It must be called
// after all assemblies have been initialized, i.e. from Prepare or Start.
func RegisterOptional(registry Resolver, register func(*Registry)) {
	if r, found := registry.ResolveOptional(RegistryKey); found {
		register(r.(*Registry))
	}
}
