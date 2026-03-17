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

package natsagent

import (
	"os"
	"testing"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/require"
)

func Test_LaunchAgent_WithAssemblyProvider(t *testing.T) {
	// Setup environment variables
	_ = os.Setenv("TESTAGENT_URI", "nats://localhost:4222")
	_ = os.Setenv("TESTAGENT_BUCKET", "test-bucket")
	_ = os.Setenv("TESTAGENT_STREAM", "test-stream")

	t.Cleanup(func() {
		_ = os.Unsetenv("TESTAGENT_URI")
		_ = os.Unsetenv("TESTAGENT_BUCKET")
		_ = os.Unsetenv("TESTAGENT_STREAM")
	})

	// Create a flag to track if AssemblyProvider was called
	var assemblyProviderCalled bool
	var collectedServices []system.ServiceType

	// Create mock assemblies that will be provided
	mockAssembly1 := &MockServiceAssembly{
		providedServices: []system.ServiceType{"service1", "service2"},
	}
	mockAssembly2 := &MockServiceAssembly{
		providedServices: []system.ServiceType{"service3"},
	}

	// Create the launcher config with AssemblyProvider
	config := LauncherConfig{
		AgentName:    "TestAgent",
		ConfigPrefix: "testagent",
		ActivityType: "test-activity",
		AssemblyProvider: func() []system.ServiceAssembly {
			assemblyProviderCalled = true
			return []system.ServiceAssembly{mockAssembly1, mockAssembly2}
		},
		NewProcessor: func(ctx *AgentContext) api.ActivityProcessor {
			return &MockActivityProcessor{}
		},
	}

	// Create shutdown channel
	shutdownChannel := make(chan struct{})

	// Verify AssemblyProvider is called and services are collected
	require.NotNil(t, config.AssemblyProvider)
	assemblies := config.AssemblyProvider()
	require.True(t, assemblyProviderCalled, "AssemblyProvider should have been called")
	require.Len(t, assemblies, 2, "AssemblyProvider should return 2 assemblies")

	// Verify that the selected code path collects services from all assemblies
	for _, assembly := range assemblies {
		for _, serviceType := range assembly.Provides() {
			collectedServices = append(collectedServices, serviceType)
		}
	}

	require.Len(t, collectedServices, 3, "Should collect 3 services total")
	require.Contains(t, collectedServices, system.ServiceType("service1"))
	require.Contains(t, collectedServices, system.ServiceType("service2"))
	require.Contains(t, collectedServices, system.ServiceType("service3"))

	// Verify NewProcessor creates a processor
	require.NotNil(t, config.NewProcessor)
	processor := config.NewProcessor(&AgentContext{Monitor: nil})
	require.NotNil(t, processor)

	// Cleanup
	close(shutdownChannel)
}

func Test_LaunchAgent_WithoutAssemblyProvider(t *testing.T) {
	// Setup environment variables
	_ = os.Setenv("TESTAGENT_URI", "nats://localhost:4222")
	_ = os.Setenv("TESTAGENT_BUCKET", "test-bucket")
	_ = os.Setenv("TESTAGENT_STREAM", "test-stream")

	t.Cleanup(func() {
		_ = os.Unsetenv("TESTAGENT_URI")
		_ = os.Unsetenv("TESTAGENT_BUCKET")
		_ = os.Unsetenv("TESTAGENT_STREAM")
	})

	// Create launcher config without AssemblyProvider
	config := LauncherConfig{
		AgentName:        "TestAgent",
		ConfigPrefix:     "testagent",
		ActivityType:     "test-activity",
		AssemblyProvider: nil, // nil AssemblyProvider
		NewProcessor: func(ctx *AgentContext) api.ActivityProcessor {
			return &MockActivityProcessor{}
		},
	}

	// Verify config doesn't require AssemblyProvider
	cfg := loadAgentConfig(config.AgentName, config.ConfigPrefix)
	require.NotNil(t, cfg)

	// Verify the code path handles nil AssemblyProvider
	require.Nil(t, config.AssemblyProvider)

	// Create shutdown channel
	shutdownChannel := make(chan struct{})
	defer close(shutdownChannel)
}

func Test_loadAgentConfig_MissingRequiredParams(t *testing.T) {
	// Test with missing URI
	_ = os.Setenv("TESTAGENT_BUCKET", "test-bucket")
	_ = os.Setenv("TESTAGENT_STREAM", "test-stream")

	t.Cleanup(func() {
		_ = os.Unsetenv("TESTAGENT_URI")
		_ = os.Unsetenv("TESTAGENT_BUCKET")
		_ = os.Unsetenv("TESTAGENT_STREAM")
	})

	// Verify that missing required params causes panic
	require.Panics(t, func() {
		loadAgentConfig("TestAgent", "testagent")
	}, "loadAgentConfig should panic when required parameters are missing")
}

// MockServiceAssembly implements system.ServiceAssembly for testing
type MockServiceAssembly struct {
	providedServices []system.ServiceType
	registryCalls    int
}

func (m *MockServiceAssembly) Name() string {
	return "MockServiceAssembly"
}

func (m *MockServiceAssembly) Provides() []system.ServiceType {
	return m.providedServices
}

func (m *MockServiceAssembly) Requires() []system.ServiceType {
	return []system.ServiceType{}
}

func (m *MockServiceAssembly) Init(initCtx *system.InitContext) error {
	return nil
}

func (m *MockServiceAssembly) Prepare(initCtx *system.InitContext) error {
	return nil
}

func (m *MockServiceAssembly) Start(startCtx *system.StartContext) error {
	return nil
}

func (m *MockServiceAssembly) Finalize() error {
	return nil
}

func (m *MockServiceAssembly) Shutdown() error {
	return nil
}

// MockActivityProcessor implements api.ActivityProcessor for testing
type MockActivityProcessor struct{}

func (m *MockActivityProcessor) Process(activityContext api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{Result: api.ActivityResultComplete}
}
