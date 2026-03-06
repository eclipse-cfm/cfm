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

//go:generate mockery

package natsclient

import (
	"context"
	"fmt"
	"time"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultDuration = 20 * time.Second
	defaultPings    = 5
	forever         = -1

	NatsClientKey system.ServiceType = "pmapi:NatsClient"
)

type NatsClient struct {
	Connection *nats.Conn
	JetStream  jetstream.JetStream
	KVStore    jetstream.KeyValue
}

// NewNatsClient creates and returns a new NatsClient instance connected to the specified URL and bucket with given options.
// If options are not provided, default Connection settings are used for the NATS Client configuration.
// Returns an error if the Connection to NATS or JetStream initialization fails.
func NewNatsClient(url string, bucket string, options ...nats.Option) (*NatsClient, error) {
	if options == nil || len(options) == 0 {
		options = []nats.Option{nats.PingInterval(defaultDuration),
			nats.MaxPingsOutstanding(defaultPings),
			nats.ReconnectWait(time.Second),
			nats.RetryOnFailedConnect(true),
			nats.MaxReconnects(forever)}
	}
	connection, err := nats.Connect(url, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	jetStream, err := jetstream.New(connection)
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	kvManager, err := jetStream.CreateOrUpdateKeyValue(context.Background(), jetstream.KeyValueConfig{Bucket: bucket})
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("failed to create jetstream key value store with bucket %s: %w", bucket, err)
	}

	return &NatsClient{
		Connection: connection,
		JetStream:  jetStream,
		KVStore:    kvManager,
	}, nil
}

// Close closes the NATS Connection.
func (nc *NatsClient) Close() {
	if nc.Connection != nil {
		nc.Connection.Close()
	}
}

// MsgClient is an interface for interacting with NATS. This interface is used to allow for mocking in unit tests that
// verify correct behavior in response to error conditions (i.e., negative tests).
type MsgClient interface {
	Update(ctx context.Context, key string, value []byte, version uint64) (uint64, error)
	Stream(ctx context.Context, streamName string) (jetstream.Stream, error)
	Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error)
	Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
	PublishMsg(ctx context.Context, msg *nats.Msg, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
}

func NewMsgClient(nc *NatsClient) MsgClient {
	return natsClientAdapter{Client: nc}
}

// Wraps the NatsClient to satisfy the MsgClient interface.
type natsClientAdapter struct {
	Client *NatsClient
}

func (a natsClientAdapter) Update(ctx context.Context, key string, value []byte, version uint64) (uint64, error) {
	return a.Client.KVStore.Update(ctx, key, value, version)
}

func (a natsClientAdapter) Stream(ctx context.Context, streamName string) (jetstream.Stream, error) {
	return a.Client.JetStream.Stream(ctx, streamName)
}

func (a natsClientAdapter) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	return a.Client.KVStore.Get(ctx, key)
}

func (a natsClientAdapter) Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	return a.Client.JetStream.Publish(ctx, subject, payload, opts...)
}

func (a natsClientAdapter) PublishMsg(ctx context.Context, msg *nats.Msg, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	return a.Client.JetStream.PublishMsg(ctx, msg, opts...)
}
