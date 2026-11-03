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
	"testing"

	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/stretchr/testify/require"
)

// The processor has no clients, so any event that is not rejected up front would panic on the first API call.
func TestProcess_RejectsInvalidParticipantContextID(t *testing.T) {
	processor := NewProcessor(&Config{LogMonitor: system.NoopMonitor{}})

	for _, subject := range []string{"events.keypair.activated", "events.keypair.revoked"} {
		t.Run(subject, func(t *testing.T) {
			evt := lifecycleagent.EventContext[KeyManagementEvent]{
				Subject: subject,
				Payload: KeyManagementEvent{Data: KeyPairEventData{ParticipantContextID: "../other?force=true#"}},
			}
			require.ErrorContains(t, processor.Process(t.Context(), evt), "invalid participant context ID")
		})
	}
}
