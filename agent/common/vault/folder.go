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
	"fmt"
	"strings"
)

// ParticipantFolderPath returns the Vault folder path for a participant's component, e.g. "<participantContextId>/identityhub".
// Each element must be a single, non-traversing path component, so that one participant's secrets can never be pointed at
// another participant's folder.
func ParticipantFolderPath(participantContextID string, component string) (string, error) {
	for _, element := range []string{participantContextID, component} {
		if element == "" || element == "." || element == ".." || strings.ContainsAny(element, `/\`) {
			return "", fmt.Errorf("invalid vault folder path component %q", element)
		}
	}
	return participantContextID + "/" + component, nil
}
