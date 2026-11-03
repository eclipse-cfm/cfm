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

package vault

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParticipantFolderPath(t *testing.T) {
	path, err := ParticipantFolderPath("0123456789abcdef0123456789abcdef", "identityhub")
	require.NoError(t, err)
	require.Equal(t, "0123456789abcdef0123456789abcdef/identityhub", path)

	for _, id := range []string{"", ".", "..", "../other", "a/b", `a\b`} {
		_, err := ParticipantFolderPath(id, "identityhub")
		require.Error(t, err, "participant context ID %q", id)
	}
	_, err = ParticipantFolderPath("participant", "../controlplane")
	require.Error(t, err)
}
