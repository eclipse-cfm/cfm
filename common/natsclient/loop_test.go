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

package natsclient

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBatch is a jetstream.MessageBatch holding the given messages and batch error.
type fakeBatch struct {
	msgs []jetstream.Msg
	err  error
}

func (b *fakeBatch) Messages() <-chan jetstream.Msg {
	ch := make(chan jetstream.Msg, len(b.msgs))
	for _, m := range b.msgs {
		ch <- m
	}
	close(ch)
	return ch
}

func (b *fakeBatch) Error() error { return b.err }

// fakeConsumer is a jetstream.Consumer whose Fetch delegates to a function. Methods that are not overridden panic.
type fakeConsumer struct {
	jetstream.Consumer
	fetch func() (jetstream.MessageBatch, error)
}

func (c *fakeConsumer) Fetch(int, ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	return c.fetch()
}

func TestFetchLoop_RetriesFetchErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	calls := 0
	consumer := &fakeConsumer{fetch: func() (jetstream.MessageBatch, error) {
		calls++
		switch {
		case calls < 3:
			return nil, jetstream.ErrConnectionClosed
		case calls == 3:
			return &fakeBatch{msgs: []jetstream.Msg{&fakeMsg{}}}, nil
		default:
			cancel()
			return &fakeBatch{}, nil
		}
	}}

	handled := 0
	loop := &FetchLoop{RetryBackoff: time.Millisecond}
	err := loop.Run(ctx, consumer, system.NoopMonitor{}, func(context.Context, jetstream.Msg) { handled++ })

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, handled, "messages fetched after errors must still be handled")
	assert.GreaterOrEqual(t, calls, 4)
}

func TestFetchLoop_BatchErrorIsRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	loop := &FetchLoop{RetryBackoff: time.Millisecond}
	calls := 0
	var started time.Time
	consumer := &fakeConsumer{fetch: func() (jetstream.MessageBatch, error) {
		calls++
		if calls == 1 {
			started = loop.LastFetch()
		}
		if calls == 3 {
			cancel()
		}
		return &fakeBatch{err: jetstream.ErrNoHeartbeat}, nil
	}}

	err := loop.Run(ctx, consumer, system.NoopMonitor{}, func(context.Context, jetstream.Msg) {})

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 3, calls)
	assert.Equal(t, started, loop.LastFetch(), "a failed batch must not count as a successful fetch")
}

func TestIsConsumerGone(t *testing.T) {
	assert.True(t, isConsumerGone(jetstream.ErrConsumerDeleted))
	assert.True(t, isConsumerGone(fmt.Errorf("wrapped: %w", nats.ErrNoResponders)))
	assert.True(t, isConsumerGone(&jetstream.APIError{ErrorCode: jetstream.JSErrCodeConsumerNotFound}))
	assert.False(t, isConsumerGone(jetstream.ErrNoHeartbeat))
	assert.False(t, isConsumerGone(jetstream.ErrConnectionClosed))
}

func TestFetchLoop_RecreatesDeletedConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	deleted := &fakeConsumer{fetch: func() (jetstream.MessageBatch, error) {
		return &fakeBatch{err: jetstream.ErrConsumerDeleted}, nil
	}}
	recreatedCalls := 0
	recreated := &fakeConsumer{fetch: func() (jetstream.MessageBatch, error) {
		recreatedCalls++
		cancel()
		return &fakeBatch{}, nil
	}}

	factoryCalls := 0
	loop := &FetchLoop{
		RetryBackoff: time.Millisecond,
		Recreate: func(context.Context) (jetstream.Consumer, error) {
			factoryCalls++
			if factoryCalls == 1 {
				return nil, errors.New("stream leader not available")
			}
			return recreated, nil
		},
	}
	err := loop.Run(ctx, deleted, system.NoopMonitor{}, func(context.Context, jetstream.Msg) {})

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 2, factoryCalls, "a failed recreation must be retried")
	assert.Equal(t, 1, recreatedCalls, "the loop must switch to the recreated consumer")
}

func TestFetchLoop_StopsDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	consumer := &fakeConsumer{fetch: func() (jetstream.MessageBatch, error) {
		cancel()
		return nil, errors.New("fetch failed")
	}}

	loop := &FetchLoop{RetryBackoff: time.Hour}
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx, consumer, system.NoopMonitor{}, func(context.Context, jetstream.Msg) {}) }()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not stop while backing off")
	}
}

func TestFetchLoop_Check(t *testing.T) {
	loop := &FetchLoop{}
	require.ErrorContains(t, loop.Check(time.Minute), "not running")

	loop.lastFetch.Store(time.Now().UnixNano())
	require.NoError(t, loop.Check(time.Minute))

	loop.lastFetch.Store(time.Now().Add(-2 * time.Minute).UnixNano())
	require.ErrorContains(t, loop.Check(time.Minute), "has not fetched successfully")

	loop.inFlight.Store(true)
	require.NoError(t, loop.Check(time.Minute), "a loop handling a message is alive")
}

// heartbeatMsg is a jetstream.Msg that counts how often it is reported as in progress.
type heartbeatMsg struct {
	fakeMsg
	inProgress    atomic.Int32
	inProgressErr error
}

func (m *heartbeatMsg) InProgress() error {
	m.inProgress.Add(1)
	return m.inProgressErr
}

// warnCountingMonitor counts warnings.
type warnCountingMonitor struct {
	system.NoopMonitor
	warnings atomic.Int32
}

func (m *warnCountingMonitor) Warnf(string, ...any) { m.warnings.Add(1) }

func TestFetchLoop_HeartbeatsWhileHandling(t *testing.T) {
	msg := &heartbeatMsg{}
	loop := &FetchLoop{HeartbeatInterval: 10 * time.Millisecond}

	var duringHandling int32
	loop.handleWithHeartbeat(t.Context(), msg, system.NoopMonitor{}, func(context.Context, jetstream.Msg) {
		time.Sleep(55 * time.Millisecond)
		duringHandling = msg.inProgress.Load()
	})

	assert.GreaterOrEqual(t, duringHandling, int32(3), "a slow handler must keep its message in progress")
	afterHandling := msg.inProgress.Load()
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, afterHandling, msg.inProgress.Load(), "no heartbeat may be sent once the handler has returned")
}

func TestFetchLoop_NoHeartbeatForFastHandler(t *testing.T) {
	msg := &heartbeatMsg{}
	loop := &FetchLoop{HeartbeatInterval: time.Hour}

	loop.handleWithHeartbeat(t.Context(), msg, system.NoopMonitor{}, func(context.Context, jetstream.Msg) {})

	assert.Zero(t, msg.inProgress.Load())
}

func TestFetchLoop_HeartbeatStopsWhenHandlerPanics(t *testing.T) {
	msg := &heartbeatMsg{}
	loop := &FetchLoop{HeartbeatInterval: 5 * time.Millisecond}

	assert.Panics(t, func() {
		loop.handleWithHeartbeat(t.Context(), msg, system.NoopMonitor{}, func(context.Context, jetstream.Msg) {
			time.Sleep(20 * time.Millisecond)
			panic("boom")
		})
	})
	afterPanic := msg.inProgress.Load()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, afterPanic, msg.inProgress.Load())
}

func TestFetchLoop_HeartbeatErrors(t *testing.T) {
	t.Run("already acknowledged is ignored", func(t *testing.T) {
		msg := &heartbeatMsg{inProgressErr: fmt.Errorf("wrapped: %w", jetstream.ErrMsgAlreadyAckd)}
		monitor := &warnCountingMonitor{}
		loop := &FetchLoop{HeartbeatInterval: 5 * time.Millisecond}

		loop.handleWithHeartbeat(t.Context(), msg, monitor, func(context.Context, jetstream.Msg) { time.Sleep(25 * time.Millisecond) })

		assert.Positive(t, msg.inProgress.Load())
		assert.Zero(t, monitor.warnings.Load())
	})

	t.Run("other errors are logged and the handler keeps running", func(t *testing.T) {
		msg := &heartbeatMsg{inProgressErr: jetstream.ErrConnectionClosed}
		monitor := &warnCountingMonitor{}
		loop := &FetchLoop{HeartbeatInterval: 5 * time.Millisecond}

		handled := false
		loop.handleWithHeartbeat(t.Context(), msg, monitor, func(context.Context, jetstream.Msg) {
			time.Sleep(25 * time.Millisecond)
			handled = true
		})

		assert.True(t, handled)
		assert.Positive(t, monitor.warnings.Load())
	})
}
