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
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	// DefaultMaxDeliver bounds how often a message is delivered to a consumer before it is given up on. It covers
	// explicit retries (NAKs) as well as redeliveries after an ack timeout.
	DefaultMaxDeliver = 10

	// unlimitedMaxDeliver disables the server-side delivery limit. Activity consumers use it because rescheduled
	// activities poll by redelivering the same message; their retries are bounded by the activity executor instead.
	unlimitedMaxDeliver = -1
)

// DefaultAckWait is how long the server waits for a message to be acknowledged before redelivering it, for example
// because the consumer crashed while processing it. It must be long enough for a slow message to be processed without
// being redelivered in the meantime.
//
// Consumers deliberately set no BackOff schedule. With a BackOff, NATS ignores the delay of a second NakWithDelay and
// redelivers on the BackOff schedule instead, which would break rescheduled activities. Explicit retries are delayed
// with NakWithDelay instead (see NakWithBackoff).
const DefaultAckWait = 30 * time.Second

// retryDelays is the delay before a message that failed with a retriable error is redelivered, indexed by the number
// of failed attempts so far. The last value is used for every attempt beyond the end of the schedule.
var retryDelays = []time.Duration{time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, time.Minute}

// RetryDelay returns the delay before retry number attempt (counting from 1).
func RetryDelay(attempt int) time.Duration {
	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(retryDelays) {
		idx = len(retryDelays) - 1
	}
	return retryDelays[idx]
}

// NakWithBackoff asks for the message to be redelivered after a delay that grows with the number of times it has
// already been delivered. A plain Nak would redeliver it immediately.
func NakWithBackoff(message jetstream.Msg) error {
	return message.NakWithDelay(RetryDelay(deliveryCount(message)))
}

// IsFinalDelivery reports whether the message is on its last allowed delivery, so that a failure must not be retried.
// A maxDeliver of zero or less means there is no limit.
func IsFinalDelivery(message jetstream.Msg, maxDeliver int) bool {
	return maxDeliver > 0 && deliveryCount(message) >= maxDeliver
}

// deliveryCount returns how often the message has been delivered, counting the current delivery. It returns 1 when the
// message carries no JetStream metadata.
func deliveryCount(message jetstream.Msg) int {
	md, err := message.Metadata()
	if err != nil || md == nil || md.NumDelivered == 0 {
		return 1
	}
	return int(md.NumDelivered)
}
