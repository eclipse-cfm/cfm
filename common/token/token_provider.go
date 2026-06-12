/*
 *  Copyright (c) 2025 Metaform Systems, Inc.
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

package token

import "context"

// TokenProvider gets tokens, most likely access tokens, API Keys, OAuth2 tokens, etc.
// scope is a string that describes the intended use of the token, for example, "identity-api:read"
// participantId is a string that identifies the participant for which the token is intended. Most likely, this is the participant's DID
type TokenProvider interface {
	GetToken(ctx context.Context, scope string, participantId string) (string, error)
	// todo: implement refresh
}
