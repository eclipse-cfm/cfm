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
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Configuration keys for the database connection pool, relative to the component's configuration prefix. With the
// standard environment variable mapping they translate to e.g. TM_DB_MAXOPENCONNS for the Tenant Manager.
const (
	PoolMaxOpenConnsKey    = "db.maxOpenConns"
	PoolMaxIdleConnsKey    = "db.maxIdleConns"
	PoolConnMaxLifetimeKey = "db.connMaxLifetime"
	PoolConnMaxIdleTimeKey = "db.connMaxIdleTime"
)

// Pool defaults. They keep a replica well below the max_connections of a default Postgres (100).
const (
	DefaultMaxOpenConns    = 25
	DefaultMaxIdleConns    = 10
	DefaultConnMaxLifetime = 30 * time.Minute
	DefaultConnMaxIdleTime = 5 * time.Minute
)

// PoolConfig configures the connection pool of a database handle.
type PoolConfig struct {
	// MaxOpenConns is the maximum number of open connections, in use or idle.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections kept for reuse. It can't exceed MaxOpenConns.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum time a connection is reused, so that connections are rebalanced after a database
	// failover or a change behind a load balancer.
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime is the maximum time a connection stays idle before it is closed.
	ConnMaxIdleTime time.Duration
}

// PoolFromConfig reads the pool configuration, using the defaults for keys that aren't set.
func PoolFromConfig(v *viper.Viper) (PoolConfig, error) {
	config := PoolConfig{
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
		ConnMaxIdleTime: DefaultConnMaxIdleTime,
	}
	if v.IsSet(PoolMaxOpenConnsKey) {
		config.MaxOpenConns = v.GetInt(PoolMaxOpenConnsKey)
	}
	if v.IsSet(PoolMaxIdleConnsKey) {
		config.MaxIdleConns = v.GetInt(PoolMaxIdleConnsKey)
	}
	if v.IsSet(PoolConnMaxLifetimeKey) {
		config.ConnMaxLifetime = v.GetDuration(PoolConnMaxLifetimeKey)
	}
	if v.IsSet(PoolConnMaxIdleTimeKey) {
		config.ConnMaxIdleTime = v.GetDuration(PoolConnMaxIdleTimeKey)
	}
	if err := config.Validate(); err != nil {
		return PoolConfig{}, err
	}
	return config, nil
}

// Validate checks that the pool is bounded and consistent.
func (c PoolConfig) Validate() error {
	if c.MaxOpenConns < 1 {
		return fmt.Errorf("%s must be at least 1, got %d", PoolMaxOpenConnsKey, c.MaxOpenConns)
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("%s must not be negative, got %d", PoolMaxIdleConnsKey, c.MaxIdleConns)
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("%s (%d) must not exceed %s (%d)", PoolMaxIdleConnsKey, c.MaxIdleConns, PoolMaxOpenConnsKey, c.MaxOpenConns)
	}
	if c.ConnMaxLifetime < 0 {
		return fmt.Errorf("%s must not be negative, got %s", PoolConnMaxLifetimeKey, c.ConnMaxLifetime)
	}
	if c.ConnMaxIdleTime < 0 {
		return fmt.Errorf("%s must not be negative, got %s", PoolConnMaxIdleTimeKey, c.ConnMaxIdleTime)
	}
	return nil
}

// Apply configures the pool of the given database handle.
func (c PoolConfig) Apply(db *sql.DB) {
	db.SetMaxOpenConns(c.MaxOpenConns)
	db.SetMaxIdleConns(c.MaxIdleConns)
	db.SetConnMaxLifetime(c.ConnMaxLifetime)
	db.SetConnMaxIdleTime(c.ConnMaxIdleTime)
}
