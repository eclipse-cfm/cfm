//  Copyright (c) 2026 Metaform Systems, Inc
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

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMsg is a jetstream.Msg that records how it was acknowledged. Methods that are not overridden panic.
type fakeMsg struct {
	jetstream.Msg
	data   []byte
	acked  bool
	naked  bool
	termed bool
}

func (m *fakeMsg) Data() []byte         { return m.data }
func (m *fakeMsg) Subject() string      { return "test.subject" }
func (m *fakeMsg) Headers() nats.Header { return nats.Header{} }
func (m *fakeMsg) Ack() error           { m.acked = true; return nil }
func (m *fakeMsg) Nak() error           { m.naked = true; return nil }
func (m *fakeMsg) Term() error          { m.termed = true; return nil }

func TestProcessMessage_DispatcherPanics(t *testing.T) {
	processor := &RetriableMessageProcessor[map[string]any]{
		Dispatcher: func(ctx context.Context, payload map[string]any) error {
			_ = payload["key"].(string) // type confusion on untrusted input
			return nil
		},
		Monitor: system.NoopMonitor{},
	}
	msg := &fakeMsg{data: []byte(`{"key": 1}`)}

	var err error
	require.NotPanics(t, func() {
		err = processor.ProcessMessage(t.Context(), msg)
	})

	require.ErrorContains(t, err, "panic while processing message")
	assert.True(t, msg.termed, "a message that caused a panic must be terminated")
	assert.False(t, msg.acked)
	assert.False(t, msg.naked)
}

func TestProcessMessage_Success(t *testing.T) {
	processor := &RetriableMessageProcessor[map[string]any]{
		Dispatcher: func(ctx context.Context, payload map[string]any) error {
			return nil
		},
		Monitor: system.NoopMonitor{},
	}
	msg := &fakeMsg{data: []byte(`{"key": "value"}`)}

	require.NoError(t, processor.ProcessMessage(t.Context(), msg))
	assert.True(t, msg.acked)
	assert.False(t, msg.termed)
}
