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

package natsclient

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Configuration keys for the JetStream resources a component creates, relative to the component's configuration prefix.
// With the standard environment variable mapping they translate to e.g. PM_NATS_JETSTREAM_REPLICAS for the Provision
// Manager. They only apply when a component creates a stream or the key-value bucket; existing resources are never
// changed, so resources provisioned out of band keep their configuration.
const (
	JetStreamReplicasKey       = "nats.jetstream.replicas"
	JetStreamStreamMaxAgeKey   = "nats.jetstream.stream.maxAge"
	JetStreamStreamMaxBytesKey = "nats.jetstream.stream.maxBytes"
	JetStreamKVMaxBytesKey     = "nats.jetstream.kv.maxBytes"
)

// maxReplicas is the highest replica count JetStream supports.
const maxReplicas = 5

// JetStreamConfig configures the streams and the key-value bucket a component creates when they don't exist yet. Zero
// values use the JetStream defaults: one replica and no limits.
type JetStreamConfig struct {
	// Replicas is the number of replicas of each created stream and of the key-value bucket. Use 3 on a clustered NATS.
	Replicas int
	// StreamMaxAge is the maximum age of messages in a created stream.
	StreamMaxAge time.Duration
	// StreamMaxBytes is the maximum size of a created stream in bytes.
	StreamMaxBytes int64
	// KVMaxBytes is the maximum size of the key-value bucket in bytes. The bucket holds the state of every
	// orchestration, and writes fail once it is full.
	KVMaxBytes int64
}

// Validate checks that the configured values are accepted by JetStream.
func (c JetStreamConfig) Validate() error {
	if c.Replicas < 0 || c.Replicas > maxReplicas {
		return fmt.Errorf("%s must be between 1 and %d, got %d", JetStreamReplicasKey, maxReplicas, c.Replicas)
	}
	if c.StreamMaxAge < 0 {
		return fmt.Errorf("%s must not be negative, got %s", JetStreamStreamMaxAgeKey, c.StreamMaxAge)
	}
	if c.StreamMaxBytes < 0 {
		return fmt.Errorf("%s must not be negative, got %d", JetStreamStreamMaxBytesKey, c.StreamMaxBytes)
	}
	if c.KVMaxBytes < 0 {
		return fmt.Errorf("%s must not be negative, got %d", JetStreamKVMaxBytesKey, c.KVMaxBytes)
	}
	return nil
}

// replicas returns the replica count to create resources with, defaulting to one.
func (c JetStreamConfig) replicas() int {
	if c.Replicas == 0 {
		return 1
	}
	return c.Replicas
}

// streamMaxBytes returns the stream size limit, where -1 means unlimited.
func (c JetStreamConfig) streamMaxBytes() int64 {
	if c.StreamMaxBytes == 0 {
		return -1
	}
	return c.StreamMaxBytes
}

// JetStreamFromConfig resolves the JetStream resource configuration and validates it.
func JetStreamFromConfig(v *viper.Viper) (JetStreamConfig, error) {
	config := JetStreamConfig{
		Replicas:       v.GetInt(JetStreamReplicasKey),
		StreamMaxAge:   v.GetDuration(JetStreamStreamMaxAgeKey),
		StreamMaxBytes: v.GetInt64(JetStreamStreamMaxBytesKey),
		KVMaxBytes:     v.GetInt64(JetStreamKVMaxBytesKey),
	}
	if err := config.Validate(); err != nil {
		return JetStreamConfig{}, err
	}
	return config, nil
}
