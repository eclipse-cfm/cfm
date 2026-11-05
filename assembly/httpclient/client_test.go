// Copyright (c) 2026 Metaform Systems, Inc
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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_TimesOutHangingAttempts(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()

	client := newClient(clientConfig{
		retryMax:     2,
		retryWaitMin: time.Millisecond,
		retryWaitMax: time.Millisecond,
		timeout:      50 * time.Millisecond,
	}, nil)

	start := time.Now()
	resp, err := client.Get(server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.Error(t, err, "a server that never answers must not block the caller")
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, int32(3), attempts.Load(), "an attempt that times out must be retried")
}

func TestNewClient_SucceedsWithinTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newClient(clientConfig{retryMax: 2, retryWaitMin: time.Millisecond, retryWaitMax: time.Millisecond, timeout: time.Second}, nil)

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
