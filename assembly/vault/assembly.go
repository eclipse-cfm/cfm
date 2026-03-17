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

package vault

import (
	"context"
	"fmt"

	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
)

const (
	urlKey             = "vault.url"
	clientIDKey        = "vault.clientId"
	clientSecretKey    = "vault.clientSecret"
	tokenURLKey        = "vault.tokenUrl"
	vaultPathKey       = "vault.path"
	vaultSoftDeleteKey = "vault.softDelete"
)

// VaultServiceAssembly defines an assembly that provides a client to Hashicorp Vault.
type VaultServiceAssembly struct {
	system.DefaultServiceAssembly
	client *vaultClient
}

func (v VaultServiceAssembly) Name() string {
	return "Vault"
}

func (v VaultServiceAssembly) Provides() []system.ServiceType {
	return []system.ServiceType{serviceapi.VaultKey}
}

func (v VaultServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{}
}

func (v VaultServiceAssembly) Init(ctx *system.InitContext) error {
	vaultURL := ctx.Config.GetString(urlKey)
	clientID := ctx.Config.GetString(clientIDKey)
	clientSecret := ctx.Config.GetString(clientSecretKey)
	tokenUrl := ctx.Config.GetString(tokenURLKey)
	vaultPath := ctx.Config.GetString(vaultPathKey)
	softDelete := ctx.Config.GetBool(vaultSoftDeleteKey)
	if err := runtime.CheckRequiredParams(urlKey, vaultURL, clientIDKey, clientID, clientSecretKey, clientSecret, tokenURLKey, tokenUrl); err != nil {
		return err
	}
	var err error
	var vaultOption []VaultOptions
	if vaultPath != "" {
		vaultPathOption := WithMountPath(vaultPath)
		vaultOption = append(vaultOption, vaultPathOption)
	}
	// enable/disable soft delete
	vaultOption = append(vaultOption, func(client *vaultClient) {
		client.softDelete = softDelete
	})
	v.client, err = newVaultClient(vaultURL, clientID, clientSecret, tokenUrl, ctx.LogMonitor, vaultOption...)
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %w", err)
	}

	err = v.client.init(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize Vault client: %w", err)
	}

	ctx.Registry.Register(serviceapi.VaultKey, v.client)

	return nil
}

func (v VaultServiceAssembly) Shutdown() error {
	if v.client != nil {
		err := v.client.Close()
		if err != nil {
			return fmt.Errorf("failed to close Vault client: %w", err)
		}
	}
	return nil
}
