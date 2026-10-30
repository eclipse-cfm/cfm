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

package natsorchestration

import (
	"encoding/json"
	"testing"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const panicTestActivityType = "test.panic-activity"

// panicTestMsg is a jetstream.Msg that records how it was acknowledged. Methods that are not overridden panic.
type panicTestMsg struct {
	jetstream.Msg
	data   []byte
	acked  bool
	naked  bool
	termed bool
}

func (m *panicTestMsg) Data() []byte         { return m.data }
func (m *panicTestMsg) Subject() string      { return "event.test-panic-activity" }
func (m *panicTestMsg) Headers() nats.Header { return nats.Header{} }
func (m *panicTestMsg) Ack() error           { m.acked = true; return nil }
func (m *panicTestMsg) Nak() error           { m.naked = true; return nil }
func (m *panicTestMsg) Term() error          { m.termed = true; return nil }

// panicTestEntry is a jetstream.KeyValueEntry holding a serialized orchestration.
type panicTestEntry struct {
	jetstream.KeyValueEntry
	value []byte
}

func (e panicTestEntry) Value() []byte    { return e.value }
func (e panicTestEntry) Revision() uint64 { return 1 }

func TestProcessMessage_PanicTerminatesMessage(t *testing.T) {
	// a non-string trace_context makes the executor panic on an unchecked type assertion
	orchestration := api.Orchestration{
		ID:             "test-panic",
		ProcessingData: map[string]any{"trace_context": 1},
		Steps: []api.OrchestrationStep{
			{Activities: []api.Activity{{ID: "A1", Type: panicTestActivityType}}},
		},
	}
	orchestrationBytes, err := json.Marshal(orchestration)
	require.NoError(t, err)

	mockClient := mocks.NewMockMsgClient(t)
	mockClient.EXPECT().Get(mock.Anything, "test-panic").Return(panicTestEntry{value: orchestrationBytes}, nil)

	executor := &NatsActivityExecutor{
		Client:       mockClient,
		ActivityType: panicTestActivityType,
		Monitor:      system.NoopMonitor{},
	}

	messageBytes, err := json.Marshal(api.ActivityMessage{
		OrchestrationID: "test-panic",
		Activity:        api.Activity{ID: "A1", Type: panicTestActivityType},
	})
	require.NoError(t, err)
	msg := &panicTestMsg{data: messageBytes}

	require.NotPanics(t, func() {
		err = executor.processMessage(t.Context(), msg)
	})

	require.ErrorContains(t, err, "panic while processing message")
	assert.True(t, msg.termed, "a message that caused a panic must be terminated")
	assert.False(t, msg.acked)
	assert.False(t, msg.naked)
}
