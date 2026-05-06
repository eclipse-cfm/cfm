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

package v1alpha1

import (
	"encoding/json"
	"testing"

	common "github.com/eclipse-cfm/cfm/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyRotationRequest_Serialization(t *testing.T) {
	req := &KeyRotationRequest{
		KeyPairID: "test-key-id",
	}

	jsonData, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), `"keyPairId":"test-key-id"`)
}

func TestKeyRotationRequest_Deserialization(t *testing.T) {
	jsonData := `{"keyPairId": "test-key-id", "algorithm": "EcDSA", "curve": "P-256", "gracePeriod": "P6M"}`

	var req KeyRotationRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	require.NoError(t, err)
	require.Equal(t, "test-key-id", req.KeyPairID)
	require.Equal(t, "EcDSA", req.Algorithm)
	require.Equal(t, "P-256", req.Curve)
	require.Equal(t, "P6M", req.GracePeriod.String())
}

func TestKeyRotationRequest_RoundTrip(t *testing.T) {
	gracePeriod := common.NewDuration("P1Y2M3DT4H")
	req := &KeyRotationRequest{
		KeyPairID:   "test-key-id",
		Algorithm:   "eddsa",
		Curve:       "ed25519",
		GracePeriod: &gracePeriod,
	}
	jsonData, err := json.Marshal(req)
	require.NoError(t, err)
	require.NotNil(t, jsonData)

	deser := KeyRotationRequest{}
	err = json.Unmarshal(jsonData, &deser)
	require.NoError(t, err)
	require.Equal(t, req, &deser)
}

func TestKeyRotationRequest_VerifyDefaults(t *testing.T) {
	jsonData := `{"keyPairId": "test-key-id"}`

	request := KeyRotationRequest{}
	err := json.Unmarshal([]byte(jsonData), &request)
	require.NoError(t, err)

	require.Equal(t, "eddsa", request.Algorithm)
	require.Equal(t, "ed25519", request.Curve)
	require.Equal(t, "P3M", request.GracePeriod.String())
}
