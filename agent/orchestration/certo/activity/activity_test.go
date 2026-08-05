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

package activity

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validConfig(certoURL string) *Config {
	return &Config{
		LogMonitor:    system.NoopMonitor{},
		TokenProvider: MockTokenProvider{},
		HttpClient:    &http.Client{},
		CertoURL:      certoURL,
	}
}

func newCertoServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)
	return server
}

var processingData = map[string]any{
	model.ParticipantIdentifier: "did:web:participant-abc",
	"participantContextId":      "client-456",
}

func TestCertoActivityProcessor_ProcessDeploy_WithValidData(t *testing.T) {
	server := newCertoServer(t)
	processor := NewProcessor(validConfig(server.URL))

	activityContext := api.NewActivityContext(context.Background(), "orch-1", api.Activity{
		ID:            "test-activity",
		Type:          "certo",
		Discriminator: api.DeployDiscriminator,
	}, copyOf(processingData), make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

func TestCertoActivityProcessor_ProcessDeploy_MissingParticipantID(t *testing.T) {
	processor := NewProcessor(validConfig("https://certo.invalid"))
	pd := copyOf(processingData)
	delete(pd, model.ParticipantIdentifier)

	activityContext := api.NewActivityContext(context.Background(), "orch-2", api.Activity{
		ID:            "activity-1",
		Type:          "certo",
		Discriminator: api.DeployDiscriminator,
	}, pd, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	require.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Contains(t, result.Error.Error(), "error processing Certo activity")
}

func TestCertoActivityProcessor_ProcessDeploy_MissingParticipantContextID(t *testing.T) {
	processor := NewProcessor(validConfig("https://certo.invalid"))
	pd := copyOf(processingData)
	delete(pd, "participantContextId")

	activityContext := api.NewActivityContext(context.Background(), "orch-3", api.Activity{
		ID:            "activity-2",
		Type:          "certo",
		Discriminator: api.DeployDiscriminator,
	}, pd, make(map[string]any))

	result := processor.ProcessDeploy(activityContext)

	require.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
	assert.Contains(t, result.Error.Error(), "error processing Certo activity")
}

func TestCertoActivityProcessor_ProcessDispose_Success(t *testing.T) {
	server := newCertoServer(t)
	processor := NewProcessor(validConfig(server.URL))

	activityContext := api.NewActivityContext(context.Background(), "orch-4", api.Activity{
		ID:            "activity-3",
		Type:          "certo",
		Discriminator: api.DisposeDiscriminator,
	}, copyOf(processingData), make(map[string]any))

	result := processor.ProcessDispose(activityContext)

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.NoError(t, result.Error)
}

// --- mocks ---

type MockTokenProvider struct {
	expectedError error
}

func (m MockTokenProvider) GetToken(_ context.Context, _ string, _ string) (string, error) {
	return "test-token", m.expectedError
}

func copyOf(m map[string]any) map[string]any {
	result := make(map[string]any)
	maps.Copy(result, m)
	return result
}
