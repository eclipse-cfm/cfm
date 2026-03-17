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

package serviceapi

import (
	"context"

	"github.com/eclipse-cfm/cfm/common/system"
)

const (
	VaultKey system.ServiceType = "vault:VaultClient"
)

// VaultClient defines an interface for interacting with a secure secrets vault.
type VaultClient interface {
	ResolveSecret(ctx context.Context, path string) (string, error)
	StoreSecret(ctx context.Context, path string, value string) error
	DeleteSecret(ctx context.Context, path string) error
	Close() error
}
