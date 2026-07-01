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

// KeyPairResource represents a key pair resource in the identity hub.
type KeyPairResource struct {
	KeyID               string   `json:"keyId"`
	KeyContextID        string   `json:"keyContext"`
	IsDefault           bool     `json:"defaultPair"`
	UseDuration         int      `json:"useDuration"`
	RotationDuration    int      `json:"rotationDuration"`
	SerializedPublicKey string   `json:"serializedPublicKey"`
	PrivateKeyAlias     string   `json:"privateKeyAlias"`
	Usage               []string `json:"usage"`
}

type KeyDescriptor struct {
	ResourceID      string                 `json:"resourceId"`
	KeyID           string                 `json:"keyId"`
	Type            string                 `json:"type"`
	PrivateKeyAlias string                 `json:"privateKeyAlias"`
	PublicKeyJWK    map[string]interface{} `json:"publicKeyJwk"`
	IsActive        bool                   `json:"isActive"`
	Usage           []string               `json:"usage"`
}

// KeyPairEventData is the generic event payload for all keypair-events from identity hub. All optional fields, such as
// the new key descriptor in rotate-events, or the serialized public key in create-events, are only present on those events.
type KeyPairEventData struct {
	ParticipantContextID string          `json:"participantContextId"`
	KeyID                string          `json:"keyId"`
	KeyPairResource      KeyPairResource `json:"keyPairResource"`
	// only on .added and .activated
	PublicKeySerialized string `json:"publicKeySerialized,omitempty"`
	Type                string `json:"type,omitempty"`
	// only on .rotated and .revoked
	NewKey KeyDescriptor `json:"newKeyDescriptor,omitempty"`
}
