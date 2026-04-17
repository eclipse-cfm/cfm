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

package natsfixtures

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The image used for testing
const natsImage = "nats:alpine"

type NatsTestContainer struct {
	Container testcontainers.Container
	URI       string
	Client    *natsclient.NatsClient
}

func SetupNatsContainer(ctx context.Context, bucket string) (*NatsTestContainer, error) {
	// Create NATS configuration
	natsConfig := fmt.Sprintf(`
		# Basic server configuration
		port: 4222
		monitor_port: 8222
		
		# JetStream configuration
		jetstream {
			store_dir: "/tmp/jetstream"
			max_memory_store: 64MB
			max_file_store: 512MB
		}
		
		# Enable debug/trace
		debug: true
		trace: true
		`)

	// Create temporary config file
	tmpDir, err := os.MkdirTemp("", "nats-config-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	configFile := filepath.Join(tmpDir, "nats.conf")
	if err := os.WriteFile(configFile, []byte(natsConfig), 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        natsImage,
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForExposedPort(),
			wait.ForLog(".*Server is ready.*").AsRegexp(),
		),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      configFile,
				ContainerFilePath: "/etc/nats/nats.conf",
				FileMode:          0644,
			},
		},
		Cmd: []string{"-c", "/etc/nats/nats.conf"},
		Tmpfs: map[string]string{
			"/tmp/jetstream": "size=1G", // Provide storage space
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create/start NATS container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "4222")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host IP: %w", err)
	}

	uri := fmt.Sprintf("nats://%s:%s", hostIP, mappedPort.Port())

	natsClient, err := natsclient.NewNatsClient(uri, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS client: %w", err)
	}

	return &NatsTestContainer{
		Container: container,
		URI:       uri,
		Client:    natsClient,
	}, nil
}

func TeardownNatsContainer(ctx context.Context, nt *NatsTestContainer) {
	if nt.Client != nil {
		nt.Client.Close()
	}
	if nt.Container != nil {
		err := nt.Container.Terminate(ctx)
		if err != nil {
			fmt.Println("Error terminating container: ", err)
		}
	}
}

func SetupTestStream(t *testing.T, ctx context.Context, client *natsclient.NatsClient, streamName string) jetstream.Stream {
	stream, err := natsclient.SetupStream(ctx, client, streamName)
	require.NoError(t, err)
	return stream
}

func SetupTestConsumer(t *testing.T, ctx context.Context, stream jetstream.Stream, subject string) jetstream.Consumer {
	consumer, err := natsclient.SetupConsumer(ctx, stream, subject)
	require.NoError(t, err)
	return consumer
}
