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

package routing

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/viper"
)

const (
	RouterKey system.ServiceType = "router:Router"
	key                          = "httpPort"
)

type RouterServiceAssembly struct {
	system.DefaultServiceAssembly
	server  *http.Server
	router  *chi.Mux
	monitor system.LogMonitor
	config  *viper.Viper
}

func (r *RouterServiceAssembly) Name() string {
	return "Router"
}

func (r *RouterServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{RouterKey}
}

func (r *RouterServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{}
}

func (r *RouterServiceAssembly) Init(ctx *system.InitContext) error {
	r.router = r.setupRouter(ctx.LogMonitor, ctx.Mode)
	ctx.Registry.Register(RouterKey, r.router)
	r.monitor = ctx.LogMonitor
	r.config = ctx.Config
	return nil
}

func (r *RouterServiceAssembly) Start(ctx *system.StartContext) error {
	port := r.config.GetInt(key)
	r.server = &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: r.router,
	}

	go func() {
		r.monitor.Infof("HTTP server listening on [%d]", port)
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			r.monitor.Severew("failed to start", "error", err)
		}
	}()
	return nil
}

func (r *RouterServiceAssembly) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.server.Shutdown(ctx); err != nil {
		r.monitor.Severew("Error attempting HTTP server shutdown", "error", err)
	}
	return nil
}

// SetupRouter configures and returns the HTTP router
func (r *RouterServiceAssembly) setupRouter(monitor system.LogMonitor, mode system.RuntimeMode) *chi.Mux {
	router := chi.NewRouter()

	if mode == system.DebugMode {
		router.Use(createLoggerHandler(monitor))
	}
	router.Use(middleware.Recoverer)

	return router
}

func createLoggerHandler(monitor system.LogMonitor) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				monitor.Debugw("http",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"bytes", ww.BytesWritten(),
					"duration", time.Since(start),
					"reqId", middleware.GetReqID(r.Context()),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
