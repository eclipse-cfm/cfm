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
	"time"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultRetryBackoff = time.Second
	defaultMaxBackoff   = 30 * time.Second
	fetchMaxWait        = time.Second
)

// ConsumerFactory re-obtains a JetStream consumer, creating it (and its stream) if necessary.
type ConsumerFactory func(ctx context.Context) (jetstream.Consumer, error)

// FetchLoop repeatedly fetches messages from a JetStream consumer and hands each one to a handler. It only returns
// when its context is canceled: fetch errors, for example during a JetStream leader election or while the connection
// is reconnecting, are retried with a capped exponential backoff. If the consumer or its stream has been deleted and
// Recreate is set, the consumer is re-obtained through it.
//
// Every NATS consumer is expected to have its own FetchLoop, run in its own goroutine. A loop holds the retry and
// liveness state of a single consumer, so it must not be shared: backing off after an error then only pauses that
// consumer, while the loops of other consumers keep fetching.
//
// The loop records when it last completed a fetch, so that health checks can detect a loop that is no longer
// receiving messages.
//
// While a message is being handled, the loop periodically tells the server that it is still in progress, which resets
// the consumer's AckWait. A slow handler therefore keeps its message instead of having it redelivered to another
// consumer while it is still working on it. If the process dies, the heartbeat stops and the message is redelivered
// once AckWait has passed. Handlers must bound their own run time, since the heartbeat keeps the message for as long as
// the handler runs: RetriableMessageProcessor and the activity executor do so with a deadline.
type FetchLoop struct {
	// Recreate optionally re-obtains the consumer after it has been deleted. If nil, the loop keeps retrying the
	// original consumer.
	Recreate ConsumerFactory
	// RetryBackoff is the delay after the first failed fetch; it doubles on every consecutive failure up to
	// MaxBackoff. Defaults to one second.
	RetryBackoff time.Duration
	// MaxBackoff caps the retry delay. Defaults to 30 seconds.
	MaxBackoff time.Duration
	// HeartbeatInterval is how often an in-flight message is reported as in progress. It must be well below the
	// consumer's AckWait. Defaults to a third of DefaultAckWait.
	HeartbeatInterval time.Duration

	lastFetch atomic.Int64
	inFlight  atomic.Bool
}

// Run executes the loop until ctx is canceled, returning the context's error.
func (l *FetchLoop) Run(ctx context.Context, consumer jetstream.Consumer, monitor system.LogMonitor, handle func(context.Context, jetstream.Msg)) error {
	initial := l.RetryBackoff
	if initial <= 0 {
		initial = defaultRetryBackoff
	}
	maxBackoff := l.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = defaultMaxBackoff
	}
	backoff := initial

	l.lastFetch.Store(time.Now().UnixNano())
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := l.fetch(ctx, consumer, monitor, handle)
		if err == nil {
			backoff = initial
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if l.Recreate != nil && isConsumerGone(err) {
			monitor.Warnf("JetStream consumer is gone, recreating it: %v", err)
			recreated, recreateErr := l.Recreate(ctx)
			if recreateErr == nil {
				consumer = recreated
				backoff = initial
				continue
			}
			err = fmt.Errorf("failed to recreate consumer: %w", recreateErr)
		}

		monitor.Warnf("Error fetching messages, retrying in %s: %v", backoff, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// fetch performs a single fetch round and handles the received messages. Errors raised while the batch was being
// received are only reported through the batch, so it is checked once all messages have been handled.
func (l *FetchLoop) fetch(ctx context.Context, consumer jetstream.Consumer, monitor system.LogMonitor, handle func(context.Context, jetstream.Msg)) error {
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(fetchMaxWait))
	if err != nil {
		return err
	}

	for message := range batch.Messages() {
		l.inFlight.Store(true)
		l.handleWithHeartbeat(ctx, message, monitor, handle)
		l.inFlight.Store(false)
	}

	if err := batch.Error(); err != nil {
		return err
	}
	l.lastFetch.Store(time.Now().UnixNano())
	return nil
}

// handleWithHeartbeat runs the handler and reports the message as in progress every HeartbeatInterval until the
// handler returns. It only returns once the heartbeat has stopped, so no heartbeat is sent for a message after it has
// been handled. A heartbeat that is already on its way when the handler NAKs the message with a delay can still reach
// the server after the NAK, which shortens that delay to AckWait. The window is tiny compared to the heartbeat
// interval, and the message is then merely redelivered early.
func (l *FetchLoop) handleWithHeartbeat(ctx context.Context, message jetstream.Msg, monitor system.LogMonitor, handle func(context.Context, jetstream.Msg)) {
	interval := l.HeartbeatInterval
	if interval <= 0 {
		interval = DefaultAckWait / 3
	}

	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// A heartbeat that races with the handler acknowledging the message is harmless
				if err := message.InProgress(); err != nil && !errors.Is(err, jetstream.ErrMsgAlreadyAckd) {
					monitor.Warnf("Failed to report message on %s as in progress: %v", message.Subject(), err)
				}
			}
		}
	}()
	defer func() {
		close(stop)
		<-stopped
	}()

	handle(ctx, message)
}

// LastFetch returns when the loop last completed a fetch without error, or the zero time if it has never run.
func (l *FetchLoop) LastFetch() time.Time {
	nanos := l.lastFetch.Load()
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos)
}

// Check reports an error if the loop has not completed a fetch within maxAge. A loop that is currently handling a
// message is considered alive, since handlers may legitimately take longer than maxAge.
func (l *FetchLoop) Check(maxAge time.Duration) error {
	if l.inFlight.Load() {
		return nil
	}
	last := l.LastFetch()
	if last.IsZero() {
		return errors.New("consumer loop is not running")
	}
	if age := time.Since(last); age > maxAge {
		return fmt.Errorf("consumer loop has not fetched successfully for %s", age.Round(time.Second))
	}
	return nil
}

// isConsumerGone reports whether err indicates that the consumer may no longer exist. A pull request that is pending
// when the consumer is deleted fails with ErrConsumerDeleted, but later pull requests fail with ErrNoResponders, since
// nothing serves the deleted consumer's subject. ErrNoResponders can also be transient, for example while a consumer
// leader is elected; recreating is idempotent, so it is safe in that case too.
func isConsumerGone(err error) bool {
	return errors.Is(err, jetstream.ErrConsumerDeleted) ||
		errors.Is(err, nats.ErrNoResponders) ||
		errors.Is(err, jetstream.ErrConsumerNotFound) ||
		errors.Is(err, jetstream.ErrStreamNotFound)
}
