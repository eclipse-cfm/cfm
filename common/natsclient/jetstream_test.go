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
	"context"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/nats-io/nats.go/jetstream"
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

func TestNewNatsClient_CreatesMissingBucketWithResources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	client, err := natsclient.NewNatsClient(natsclient.ClientConfig{
		URL:       nt.URI,
		Bucket:    "new-bucket",
		Resources: natsclient.JetStreamConfig{KVMaxBytes: mebibyte},
	})
	require.NoError(t, err)
	defer client.Close()

	config := streamConfig(t, ctx, nt, "KV_new-bucket")
	assert.Equal(t, int64(mebibyte), config.MaxBytes)
	assert.Equal(t, int64(1), config.MaxMsgsPerSubject, "history")
	assert.Equal(t, 1, config.Replicas)
	assert.Equal(t, jetstream.FileStorage, config.Storage)
}

func TestNewNatsClient_KeepsExistingBucketConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	// the bucket is provisioned out of band with its own configuration
	_, err = nt.Client.JetStream.UpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:   "cfm-bucket",
		History:  5,
		TTL:      time.Hour,
		MaxBytes: mebibyte,
	})
	require.NoError(t, err)

	client, err := natsclient.NewNatsClient(natsclient.ClientConfig{
		URL:       nt.URI,
		Bucket:    "cfm-bucket",
		Resources: natsclient.JetStreamConfig{KVMaxBytes: 2 * mebibyte},
	})
	require.NoError(t, err)
	defer client.Close()

	config := streamConfig(t, ctx, nt, "KV_cfm-bucket")
	assert.Equal(t, int64(5), config.MaxMsgsPerSubject, "history")
	assert.Equal(t, time.Hour, config.MaxAge)
	assert.Equal(t, int64(mebibyte), config.MaxBytes)

	// the client still works against the existing bucket
	_, err = client.KVStore.Put(ctx, "key", []byte("value"))
	require.NoError(t, err)
}

func TestSetupStream_CreatesMissingStreamWithResources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	client := clientWithResources(t, nt, natsclient.JetStreamConfig{StreamMaxAge: time.Hour, StreamMaxBytes: mebibyte})

	_, err = natsclient.SetupStream(ctx, client, "cfm-stream")
	require.NoError(t, err)

	config := streamConfig(t, ctx, nt, "cfm-stream")
	assert.Equal(t, jetstream.WorkQueuePolicy, config.Retention)
	assert.Equal(t, jetstream.FileStorage, config.Storage)
	assert.Equal(t, 1, config.Replicas)
	assert.Equal(t, time.Hour, config.MaxAge)
	assert.Equal(t, int64(mebibyte), config.MaxBytes)
}

func TestSetupStream_KeepsExistingStreamConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	_, err = nt.Client.JetStream.CreateStream(ctx, jetstream.StreamConfig{
		Name:      "cfm-stream",
		Retention: jetstream.WorkQueuePolicy,
		Subjects:  []string{natsclient.CFMSubjectPrefix + ".*"},
		MaxAge:    2 * time.Hour,
	})
	require.NoError(t, err)

	client := clientWithResources(t, nt, natsclient.JetStreamConfig{StreamMaxAge: time.Hour, StreamMaxBytes: mebibyte})

	_, err = natsclient.SetupStream(ctx, client, "cfm-stream")
	require.NoError(t, err)

	config := streamConfig(t, ctx, nt, "cfm-stream")
	assert.Equal(t, 2*time.Hour, config.MaxAge)
	assert.Equal(t, int64(-1), config.MaxBytes)
}

func TestGetStream_CreatesFileBackedStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), streamTestTimeout)
	defer cancel()

	nt, err := natsfixtures.SetupNatsContainer(ctx, "cfm-bucket")
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	client := clientWithResources(t, nt, natsclient.JetStreamConfig{StreamMaxBytes: mebibyte})

	_, err = natsclient.GetStream(ctx, client, "events-stream", []string{"events.keypair.>"})
	require.NoError(t, err)

	config := streamConfig(t, ctx, nt, "events-stream")
	assert.Equal(t, jetstream.InterestPolicy, config.Retention)
	assert.Equal(t, jetstream.FileStorage, config.Storage)
	assert.Equal(t, int64(mebibyte), config.MaxBytes)
	assert.Equal(t, []string{"events.keypair.>"}, config.Subjects)
}

func clientWithResources(t *testing.T, nt *natsfixtures.NatsTestContainer, resources natsclient.JetStreamConfig) *natsclient.NatsClient {
	t.Helper()
	client, err := natsclient.NewNatsClient(natsclient.ClientConfig{URL: nt.URI, Bucket: "cfm-bucket", Resources: resources})
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func streamConfig(t *testing.T, ctx context.Context, nt *natsfixtures.NatsTestContainer, name string) jetstream.StreamConfig {
	t.Helper()
	stream, err := nt.Client.JetStream.Stream(ctx, name)
	require.NoError(t, err)
	info, err := stream.Info(ctx)
	require.NoError(t, err)
	return info.Config
}
