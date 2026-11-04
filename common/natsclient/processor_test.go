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
	"errors"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMsg is a jetstream.Msg that records how it was acknowledged. Methods that are not overridden panic.
type fakeMsg struct {
	jetstream.Msg
	data         []byte
	numDelivered uint64
	acked        bool
	naked        bool
	nakDelay     time.Duration
	termed       bool
}

func (m *fakeMsg) Data() []byte         { return m.data }
func (m *fakeMsg) Subject() string      { return "test.subject" }
func (m *fakeMsg) Headers() nats.Header { return nats.Header{} }
func (m *fakeMsg) Ack() error           { m.acked = true; return nil }
func (m *fakeMsg) Nak() error           { m.naked = true; return nil }
func (m *fakeMsg) Term() error          { m.termed = true; return nil }
func (m *fakeMsg) NakWithDelay(delay time.Duration) error {
	m.naked = true
	m.nakDelay = delay
	return nil
}
func (m *fakeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{NumDelivered: m.numDelivered}, nil
}

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

func newFailingProcessor(err error) *RetriableMessageProcessor[map[string]any] {
	return &RetriableMessageProcessor[map[string]any]{
		Dispatcher: func(ctx context.Context, payload map[string]any) error { return err },
		Monitor:    system.NoopMonitor{},
	}
}

func TestProcessMessage_RecoverableErrorIsRetriedWithBackoff(t *testing.T) {
	processor := newFailingProcessor(types.NewRecoverableError("downstream unavailable"))

	first := &fakeMsg{data: []byte(`{}`), numDelivered: 1}
	require.ErrorContains(t, processor.ProcessMessage(t.Context(), first), "retriable failure")
	assert.True(t, first.naked)
	assert.Equal(t, time.Second, first.nakDelay)
	assert.False(t, first.acked)

	later := &fakeMsg{data: []byte(`{}`), numDelivered: 4}
	require.Error(t, processor.ProcessMessage(t.Context(), later))
	assert.Equal(t, 10*time.Second, later.nakDelay, "the delay grows with the number of deliveries")
}

func TestProcessMessage_RecoverableErrorOnFinalDeliveryIsDeadLettered(t *testing.T) {
	processor := newFailingProcessor(types.NewRecoverableError("downstream unavailable"))
	msg := &fakeMsg{data: []byte(`{}`), numDelivered: DefaultMaxDeliver}

	require.ErrorContains(t, processor.ProcessMessage(t.Context(), msg), "dead-lettered")
	assert.True(t, msg.termed)
	assert.False(t, msg.naked)
	assert.False(t, msg.acked)
}

func TestProcessMessage_MaxDeliverOverride(t *testing.T) {
	processor := newFailingProcessor(types.NewRecoverableError("downstream unavailable"))
	processor.MaxDeliver = 3

	msg := &fakeMsg{data: []byte(`{}`), numDelivered: 3}
	require.Error(t, processor.ProcessMessage(t.Context(), msg))
	assert.True(t, msg.termed)
}

func TestProcessMessage_PermanentErrorsAreAcked(t *testing.T) {
	for name, err := range map[string]error{
		"invalid input": types.ErrInvalidInput,
		"not found":     types.ErrNotFound,
		"conflict":      types.ErrConflict,
		"client error":  types.NewClientError("bad request"),
		"fatal error":   types.NewFatalError("broken"),
	} {
		t.Run(name, func(t *testing.T) {
			msg := &fakeMsg{data: []byte(`{}`), numDelivered: 1}
			require.ErrorContains(t, newFailingProcessor(err).ProcessMessage(t.Context(), msg), "fatal failure")
			assert.True(t, msg.acked)
			assert.False(t, msg.naked, "a permanent error must not be retried")
		})
	}
}

func TestProcessMessage_UnclassifiedErrorIsRetried(t *testing.T) {
	msg := &fakeMsg{data: []byte(`{}`), numDelivered: 1}
	require.ErrorContains(t, newFailingProcessor(errors.New("boom")).ProcessMessage(t.Context(), msg), "retriable failure")
	assert.True(t, msg.naked)
	assert.False(t, msg.acked)
}

func TestProcessMessage_RecoverableWrappingPermanentErrorIsRetried(t *testing.T) {
	err := types.NewRecoverableWrappedError(types.ErrNotFound, "config not provisioned yet")
	msg := &fakeMsg{data: []byte(`{}`), numDelivered: 1}
	require.Error(t, newFailingProcessor(err).ProcessMessage(t.Context(), msg))
	assert.True(t, msg.naked)
}
