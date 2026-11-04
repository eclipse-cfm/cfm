/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/common/siglet"
	"github.com/eclipse-cfm/cfm/common/httpx"
	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSiglet is a siglet.ManagementAPIClient that returns err from every call.
type fakeSiglet struct {
	siglet.ManagementAPIClient
	err error
}

func (f fakeSiglet) GetKeyMapping(context.Context, string) (*siglet.KeyMapping, error) {
	return nil, f.err
}
func (f fakeSiglet) CreateKeyMapping(context.Context, siglet.KeyMapping) error { return f.err }
func (f fakeSiglet) UpdateKeyMapping(context.Context, siglet.KeyMapping) error { return f.err }
func (f fakeSiglet) DeleteKeyMapping(context.Context, string) error            { return f.err }

// fakeControlPlane is a controlplane.ManagementAPIClient whose PatchConfig returns err.
type fakeControlPlane struct {
	controlplane.ManagementAPIClient
	err error
}

func (f fakeControlPlane) PatchConfig(context.Context, string, controlplane.ParticipantContextConfig) error {
	return f.err
}

func keyEvent(subject string) lifecycleagent.EventContext[KeyManagementEvent] {
	return lifecycleagent.EventContext[KeyManagementEvent]{
		Subject: subject,
		Payload: KeyManagementEvent{Data: KeyPairEventData{ParticipantContextID: "participant-1", KeyID: "key-1"}},
	}
}

// A revocation or activation that fails because Siglet is unavailable must be retried, not dropped: a dropped
// revocation leaves the revoked key in use.
func TestProcess_SigletUnavailableIsRecoverable(t *testing.T) {
	unavailable := &httpx.StatusError{Operation: "siglet", StatusCode: http.StatusServiceUnavailable}
	processor := NewProcessor(&Config{
		LogMonitor:   system.NoopMonitor{},
		SigletClient: fakeSiglet{err: unavailable},
	})

	for _, subject := range []string{"events.keypair.revoked", "events.keypair.activated"} {
		t.Run(subject, func(t *testing.T) {
			err := processor.Process(t.Context(), keyEvent(subject))
			require.Error(t, err)
			assert.True(t, types.IsRecoverable(err), "%v", err)
		})
	}
}

// The participant config may not be provisioned yet when the activation arrives, so a 404 is retried.
func TestProcess_ParticipantConfigNotFoundIsRecoverable(t *testing.T) {
	processor := NewProcessor(&Config{
		LogMonitor:         system.NoopMonitor{},
		SigletClient:       fakeSiglet{},
		ControlPlaneClient: fakeControlPlane{err: &httpx.StatusError{Operation: "patch", StatusCode: http.StatusNotFound}},
	})

	err := processor.Process(t.Context(), keyEvent("events.keypair.activated"))
	require.Error(t, err)
	assert.True(t, types.IsRecoverable(err), "%v", err)
}

func TestProcess_RejectedRequestIsNotRecoverable(t *testing.T) {
	processor := NewProcessor(&Config{
		LogMonitor:   system.NoopMonitor{},
		SigletClient: fakeSiglet{err: &httpx.StatusError{Operation: "siglet", StatusCode: http.StatusBadRequest}},
	})

	err := processor.Process(t.Context(), keyEvent("events.keypair.revoked"))
	require.Error(t, err)
	assert.False(t, types.IsRecoverable(err), "%v", err)
}

func TestProcess_InvalidParticipantContextIDIsClientError(t *testing.T) {
	processor := NewProcessor(&Config{LogMonitor: system.NoopMonitor{}})
	evt := keyEvent("events.keypair.revoked")
	evt.Payload.Data.ParticipantContextID = "../other"

	err := processor.Process(t.Context(), evt)
	assert.True(t, types.IsClientError(err), "%v", err)
	assert.False(t, types.IsRecoverable(err), "%v", err)
}
