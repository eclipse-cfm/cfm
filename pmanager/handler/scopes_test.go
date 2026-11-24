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

package handler

// the scopes without a namespace, which grant their permission in the default namespace
const (
	scopePmRead  = "provision-manager-api:read"
	scopePmWrite = "provision-manager-api:write"
	scopePmAdmin = "provision-manager-api:admin"
)
