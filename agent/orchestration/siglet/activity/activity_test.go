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

package activity

import (
	"context"
	"fmt"
	"testing"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/common/siglet"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ConfigOptions func(*Config)

func WithSiglet(client siglet.TransferTypeMappingClient) ConfigOptions {
	return func(config *Config) { config.TransferTypeMappingClient = client }
}

func WithDataPlane(client controlplane.DataPlaneRegistrationClient) ConfigOptions {
	return func(config *Config) { config.DataPlaneClient = client }
}

func validConfig(opts ...ConfigOptions) *Config {
	c := Config{
		LogMonitor:                system.NoopMonitor{},
		SigletSignalingURL:        "http://siglet.edc-v.svc.cluster.local:8081",
		TransferTypeMappingClient: &MockSigletClient{},
		DataPlaneClient:           &MockDataPlaneClient{},
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &c
}

func mappingsProps() map[string]any {
	return map[string]any{
		TransferTypeMappingsKey: map[string]any{
			"HttpData-PULL": map[string]any{
				"transferType": "HttpData-PULL",
				"endpointType": "HTTP",
				"tokenSource":  "provider",
				"endpoint":     "https://data.provider.example.com/assets",
			},
		},
	}
}

// validVpaData returns a VPA data slice with a single dataplane entry. If properties is nil, the
// entry has no properties field.
func validVpaData(properties map[string]any) []any {
	entry := map[string]any{
		"vpaType": model.DataPlaneType.String(),
	}
	if properties != nil {
		entry["properties"] = properties
	}
	return []any{entry}
}

func processingDataWith(vpaData []any) map[string]any {
	pd := map[string]any{
		"participantContextId": "participant-1",
	}
	if vpaData != nil {
		pd[model.VPAData] = vpaData
	}
	return pd
}

func newContext(pd map[string]any, discriminator api.Discriminator) api.ActivityContext {
	activity := api.Activity{ID: "test-activity", Type: "siglet", Discriminator: discriminator}
	return api.NewActivityContext(context.Background(), "orch-123", activity, pd, make(map[string]any))
}

func TestSiglet_Deploy_HappyPath(t *testing.T) {
	sigletClient := &MockSigletClient{}
	dpClient := &MockDataPlaneClient{}
	processor := NewProcessor(validConfig(WithSiglet(sigletClient), WithDataPlane(dpClient)))

	result := processor.ProcessDeploy(newContext(processingDataWith(validVpaData(mappingsProps())), api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
	assert.True(t, sigletClient.created, "expected a transfer type mapping to be created")
	require.NotNil(t, sigletClient.lastMapping)
	assert.Equal(t, "participant-1", sigletClient.lastMapping.ParticipantContextID)
	assert.Contains(t, sigletClient.lastMapping.Mappings, "HttpData-PULL")
	assert.True(t, dpClient.registered, "expected the data plane to be registered")
	assert.Equal(t, "participant-1-siglet", dpClient.lastRegistration.ID)
	assert.Equal(t, []string{"HttpData-PULL"}, dpClient.lastRegistration.TransferTypes)
	assert.Equal(t, "http://siglet.edc-v.svc.cluster.local:8081/api/v1/participant-1/dataflows", dpClient.lastRegistration.Endpoint)
}

func TestSiglet_Deploy_DefaultsTokenSourceAndRenewal(t *testing.T) {
	sigletClient := &MockSigletClient{}
	processor := NewProcessor(validConfig(WithSiglet(sigletClient)))

	props := map[string]any{
		TransferTypeMappingsKey: map[string]any{
			// tokenSource omitted -> should default to "provider"; txRenewalSupport omitted -> false
			"HttpData-PULL": map[string]any{
				"transferType": "HttpData-PULL",
				"endpointType": "HTTP",
				"endpoint":     "https://data.provider.example.com/assets",
			},
		},
	}

	result := processor.ProcessDeploy(newContext(processingDataWith(validVpaData(props)), api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	require.NotNil(t, sigletClient.lastMapping)
	tt := sigletClient.lastMapping.Mappings["HttpData-PULL"]
	assert.Equal(t, "provider", tt.TokenSource)
	assert.False(t, tt.TxRenewalSupport)
}

func TestSiglet_Deploy_UpsertReplacesExisting(t *testing.T) {
	sigletClient := &MockSigletClient{existing: &siglet.TransferTypeMapping{ParticipantContextID: "participant-1"}}
	processor := NewProcessor(validConfig(WithSiglet(sigletClient)))

	result := processor.ProcessDeploy(newContext(processingDataWith(validVpaData(mappingsProps())), api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.True(t, sigletClient.replaced, "expected an existing mapping to be replaced")
	assert.False(t, sigletClient.created, "expected create not to be called when a mapping exists")
}

func TestSiglet_Deploy_NoDataPlaneVpa_CompletesDoingNothing(t *testing.T) {
	sigletClient := &MockSigletClient{}
	dpClient := &MockDataPlaneClient{}
	processor := NewProcessor(validConfig(WithSiglet(sigletClient), WithDataPlane(dpClient)))

	result := processor.ProcessDeploy(newContext(processingDataWith(nil), api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
	assert.False(t, sigletClient.created)
	assert.False(t, sigletClient.replaced)
	assert.False(t, dpClient.registered)
}

func TestSiglet_Deploy_NoMappings_CompletesDoingNothing(t *testing.T) {
	sigletClient := &MockSigletClient{}
	dpClient := &MockDataPlaneClient{}
	processor := NewProcessor(validConfig(WithSiglet(sigletClient), WithDataPlane(dpClient)))

	// dataplane VPA present but no transfer-type mappings property
	result := processor.ProcessDeploy(newContext(processingDataWith(validVpaData(map[string]any{})), api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.False(t, sigletClient.created)
	assert.False(t, dpClient.registered)
}

func TestSiglet_Deploy_MissingParticipantContextId(t *testing.T) {
	processor := NewProcessor(validConfig())

	pd := processingDataWith(validVpaData(mappingsProps()))
	delete(pd, "participantContextId")

	result := processor.ProcessDeploy(newContext(pd, api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "orch-123")
}

func TestSiglet_Deploy_SigletError(t *testing.T) {
	processor := NewProcessor(validConfig(WithSiglet(&MockSigletClient{createErr: fmt.Errorf("siglet boom")})))

	result := processor.ProcessDeploy(newContext(processingDataWith(validVpaData(mappingsProps())), api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "siglet boom")
}

func TestSiglet_Deploy_ControlPlaneError(t *testing.T) {
	processor := NewProcessor(validConfig(WithDataPlane(&MockDataPlaneClient{registerErr: fmt.Errorf("cp boom")})))

	result := processor.ProcessDeploy(newContext(processingDataWith(validVpaData(mappingsProps())), api.DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.ErrorContains(t, result.Error, "cp boom")
}

func TestSiglet_Dispose_HappyPath(t *testing.T) {
	sigletClient := &MockSigletClient{}
	dpClient := &MockDataPlaneClient{}
	processor := NewProcessor(validConfig(WithSiglet(sigletClient), WithDataPlane(dpClient)))

	result := processor.ProcessDispose(newContext(processingDataWith(nil), api.DisposeDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.True(t, sigletClient.deleted)
	assert.True(t, dpClient.unregistered)
	assert.Equal(t, "participant-1-siglet", dpClient.lastDataPlaneID)
}

func TestSiglet_Dispose_ErrorsStillComplete(t *testing.T) {
	processor := NewProcessor(validConfig(
		WithSiglet(&MockSigletClient{deleteErr: fmt.Errorf("siglet boom")}),
		WithDataPlane(&MockDataPlaneClient{unregisterErr: fmt.Errorf("cp boom")}),
	))

	result := processor.ProcessDispose(newContext(processingDataWith(nil), api.DisposeDiscriminator))

	// dispose must complete even when downstream clients error, so sibling rollback isn't blocked
	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
}

func TestSiglet_Dispose_MissingParticipantContextId(t *testing.T) {
	processor := NewProcessor(validConfig())

	result := processor.ProcessDispose(newContext(map[string]any{}, api.DisposeDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	require.Error(t, result.Error)
}

// --- mocks ---

type MockSigletClient struct {
	existing    *siglet.TransferTypeMapping
	createErr   error
	replaceErr  error
	getErr      error
	deleteErr   error
	created     bool
	replaced    bool
	deleted     bool
	lastMapping *siglet.TransferTypeMapping
}

func (m *MockSigletClient) CreateTransferTypeMapping(_ context.Context, mapping siglet.TransferTypeMapping) error {
	m.created = true
	m.lastMapping = &mapping
	return m.createErr
}

func (m *MockSigletClient) GetTransferTypeMapping(_ context.Context, _ string) (*siglet.TransferTypeMapping, error) {
	return m.existing, m.getErr
}

func (m *MockSigletClient) ReplaceTransferTypeMapping(_ context.Context, mapping siglet.TransferTypeMapping) error {
	m.replaced = true
	m.lastMapping = &mapping
	return m.replaceErr
}

func (m *MockSigletClient) DeleteTransferTypeMapping(_ context.Context, _ string) error {
	m.deleted = true
	return m.deleteErr
}

type MockDataPlaneClient struct {
	registerErr      error
	unregisterErr    error
	registered       bool
	unregistered     bool
	lastDataPlaneID  string
	lastRegistration controlplane.DataPlaneRegistration
}

func (m *MockDataPlaneClient) RegisterDataPlane(_ context.Context, _ string, registration controlplane.DataPlaneRegistration) error {
	m.registered = true
	m.lastRegistration = registration
	m.lastDataPlaneID = registration.ID
	return m.registerErr
}

func (m *MockDataPlaneClient) UnregisterDataPlane(_ context.Context, _ string, dataPlaneID string) error {
	m.unregistered = true
	m.lastDataPlaneID = dataPlaneID
	return m.unregisterErr
}
