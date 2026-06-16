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

package vault

// Config defines configuration for accessing a vault, including its URL, secret path, and folder path.
type Config struct {
	// VaultURL the base URL of the vault
	VaultURL string `json:"vaultUrl"`
	// SecretPath the path of the mount point of the secret engine
	SecretPath string `json:"secretPath"`
	// FolderPath the path of the folder within the secret engine where the participant's manifest will be stored.
	FolderPath string `json:"folderPath"`
}
