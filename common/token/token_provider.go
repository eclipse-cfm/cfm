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
type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
	// todo: implement refresh
}
