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

package sqlstore

import (
	"database/sql"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPoolFromConfig_Defaults(t *testing.T) {
	config, err := PoolFromConfig(viper.New())

	require.NoError(t, err)
	assert.Equal(t, PoolConfig{
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
		ConnMaxIdleTime: DefaultConnMaxIdleTime,
	}, config)
}

func TestPoolFromConfig_Values(t *testing.T) {
	v := viper.New()
	v.Set(PoolMaxOpenConnsKey, 8)
	v.Set(PoolMaxIdleConnsKey, 0)
	v.Set(PoolConnMaxLifetimeKey, "1h")
	v.Set(PoolConnMaxIdleTimeKey, "0s")

	config, err := PoolFromConfig(v)

	require.NoError(t, err)
	assert.Equal(t, PoolConfig{MaxOpenConns: 8, MaxIdleConns: 0, ConnMaxLifetime: time.Hour, ConnMaxIdleTime: 0}, config)
}

func TestPoolFromConfig_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
	}{
		{"no open connections", PoolMaxOpenConnsKey, 0},
		{"negative idle connections", PoolMaxIdleConnsKey, -1},
		{"more idle than open connections", PoolMaxIdleConnsKey, DefaultMaxOpenConns + 1},
		{"negative lifetime", PoolConnMaxLifetimeKey, "-1m"},
		{"negative idle time", PoolConnMaxIdleTimeKey, "-1m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			v.Set(tt.key, tt.value)

			_, err := PoolFromConfig(v)

			assert.ErrorContains(t, err, tt.key)
		})
	}
}

func TestPoolConfig_Apply(t *testing.T) {
	config := PoolConfig{MaxOpenConns: 3, MaxIdleConns: 2, ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Second}

	db, err := sql.Open("postgres", testDSN)
	require.NoError(t, err)
	defer db.Close()

	config.Apply(db)

	assert.Equal(t, 3, db.Stats().MaxOpenConnections)
}
