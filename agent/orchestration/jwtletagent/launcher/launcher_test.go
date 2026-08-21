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

package launcher

import (
	"testing"

	"github.com/eclipse-cfm/cfm/agent/orchestration/jwtletagent/activity"
	"github.com/eclipse-cfm/cfm/common/fixtures"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseClientMappings_UnsetFallsBackToDefaults asserts that an unconfigured deployment yields no
// client mappings, which is what makes the activity fall back to its built-in defaults.
func TestParseClientMappings_UnsetFallsBackToDefaults(t *testing.T) {
	mappings, err := parseClientMappings(viper.New())

	require.NoError(t, err)
	assert.Nil(t, mappings)
}

// TestParseClientMappings_StructuredList covers the config-file form, where the key holds a list of
// maps rather than a string.
func TestParseClientMappings_StructuredList(t *testing.T) {
	v := viper.New()
	v.Set(clientMappingsKey, []map[string]any{
		{"name": "controlplane", "scopes": []string{"read", "signaling"}},
		{"name": "identityhub", "scopes": []string{"read"}},
	})

	mappings, err := parseClientMappings(v)

	require.NoError(t, err)
	assert.Equal(t, []activity.ClientMapping{
		{Name: "controlplane", Scopes: []string{"read", "signaling"}},
		{Name: "identityhub", Scopes: []string{"read"}},
	}, mappings)
}

// TestParseClientMappings_JSONString covers the single-value form, which is how the key is supplied
// through an environment variable.
func TestParseClientMappings_JSONString(t *testing.T) {
	v := viper.New()
	v.Set(clientMappingsKey, `[{"name":"controlplane","scopes":["read","signaling"]},{"name":"identityhub","scopes":["read"]}]`)

	mappings, err := parseClientMappings(v)

	require.NoError(t, err)
	assert.Equal(t, []activity.ClientMapping{
		{Name: "controlplane", Scopes: []string{"read", "signaling"}},
		{Name: "identityhub", Scopes: []string{"read"}},
	}, mappings)
}

// TestParseClientMappings_FromEnvironment asserts the key is reachable through a single environment
// variable resolved by the real config loader, which is how the agent is deployed.
func TestParseClientMappings_FromEnvironment(t *testing.T) {
	fixtures.IsolateConfig(t)
	t.Setenv("JWTLETAGENT_CLIENTMAPPINGS", `[{"name":"controlplane","scopes":["read","signaling"]},{"name":"siglet-sa","scopes":["read"]}]`)

	mappings, err := parseClientMappings(system.LoadConfigOrPanic("jwtletagent"))

	require.NoError(t, err)
	assert.Equal(t, []activity.ClientMapping{
		{Name: "controlplane", Scopes: []string{"read", "signaling"}},
		{Name: "siglet-sa", Scopes: []string{"read"}},
	}, mappings)
}

// TestParseClientMappings_EmptyEnvironmentValueFallsBack asserts an empty environment variable is
// treated as unset rather than as an empty list, so it falls back to the defaults instead of failing.
func TestParseClientMappings_EmptyEnvironmentValueFallsBack(t *testing.T) {
	fixtures.IsolateConfig(t)
	t.Setenv("JWTLETAGENT_CLIENTMAPPINGS", "")

	mappings, err := parseClientMappings(system.LoadConfigOrPanic("jwtletagent"))

	require.NoError(t, err)
	assert.Nil(t, mappings)
}

// TestParseClientMappings_Invalid asserts misconfigurations fail at launch, where they are reported
// once, rather than per participant deployment.
func TestParseClientMappings_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		errContains string
	}{
		{"not json", "controlplane:read", "is not a JSON list of client mappings"},
		{"json object rather than list", `{"name":"controlplane","scopes":["read"]}`, "is not a JSON list of client mappings"},
		{"no scopes", `[{"name":"controlplane"}]`, `"controlplane" has no scopes`},
		{"empty name", `[{"scopes":["read"]}]`, "service account name is empty"},
		{"duplicate name", `[{"name":"cp","scopes":["read"]},{"name":"cp","scopes":["write"]}]`, "mapped more than once"},
		{"empty json list", "[]", "no client mappings configured"},
		{"empty structured list", []map[string]any{}, "no client mappings configured"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := viper.New()
			v.Set(clientMappingsKey, test.value)

			mappings, err := parseClientMappings(v)

			require.Error(t, err)
			assert.Nil(t, mappings)
			assert.Contains(t, err.Error(), clientMappingsKey)
			assert.Contains(t, err.Error(), test.errContains)
		})
	}
}
