//  Copyright (c) 2026 Metaform Systems, Inc
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

package natsclient_test

import (
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mebibyte = 1 << 20

func TestJetStreamFromConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		config, err := natsclient.JetStreamFromConfig(viper.New())
		require.NoError(t, err)
		assert.Equal(t, natsclient.JetStreamConfig{}, config)
	})

	t.Run("values", func(t *testing.T) {
		v := viper.New()
		v.Set(natsclient.JetStreamReplicasKey, 3)
		v.Set(natsclient.JetStreamStreamMaxAgeKey, "168h")
		v.Set(natsclient.JetStreamStreamMaxBytesKey, "1048576")
		v.Set(natsclient.JetStreamKVMaxBytesKey, 2*mebibyte)

		config, err := natsclient.JetStreamFromConfig(v)
		require.NoError(t, err)
		assert.Equal(t, natsclient.JetStreamConfig{
			Replicas:       3,
			StreamMaxAge:   168 * time.Hour,
			StreamMaxBytes: mebibyte,
			KVMaxBytes:     2 * mebibyte,
		}, config)
	})

	invalid := map[string]any{
		natsclient.JetStreamReplicasKey:       6,
		natsclient.JetStreamStreamMaxAgeKey:   "-1h",
		natsclient.JetStreamStreamMaxBytesKey: -1,
		natsclient.JetStreamKVMaxBytesKey:     -1,
	}
	for key, value := range invalid {
		t.Run("invalid "+key, func(t *testing.T) {
			v := viper.New()
			v.Set(key, value)
			_, err := natsclient.JetStreamFromConfig(v)
			require.ErrorContains(t, err, key)
		})
	}
}
