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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryDelay(t *testing.T) {
	assert.Equal(t, time.Second, RetryDelay(0))
	assert.Equal(t, time.Second, RetryDelay(1))
	assert.Equal(t, 2*time.Second, RetryDelay(2))
	assert.Equal(t, time.Minute, RetryDelay(6))
	assert.Equal(t, time.Minute, RetryDelay(100), "the last delay is kept beyond the end of the schedule")
}

func TestIsFinalDelivery(t *testing.T) {
	assert.False(t, IsFinalDelivery(&fakeMsg{numDelivered: 9}, 10))
	assert.True(t, IsFinalDelivery(&fakeMsg{numDelivered: 10}, 10))
	assert.True(t, IsFinalDelivery(&fakeMsg{numDelivered: 11}, 10))
	assert.False(t, IsFinalDelivery(&fakeMsg{numDelivered: 1000}, -1), "a negative limit means unlimited")
	assert.False(t, IsFinalDelivery(&fakeMsg{numDelivered: 0}, 10), "a message without metadata counts as its first delivery")
}
