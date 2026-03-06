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

package api

import (
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToVPAMap(t *testing.T) {
	t.Run("converts string keys to VPAType keys", func(t *testing.T) {
		vpaProperties := map[string]map[string]any{
			string(model.ConnectorType): {
				"endpoint": "https://connector.example.com",
				"port":     8080,
			},
			string(model.CredentialServiceType): {
				"keystore": "/path/to/keystore",
				"enabled":  true,
			},
		}

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 2)

		// Verify connector properties
		connectorProps, exists := (*result)[model.ConnectorType]
		require.True(t, exists)
		assert.Equal(t, "https://connector.example.com", connectorProps["endpoint"])
		assert.Equal(t, 8080, connectorProps["port"])

		// Verify credential service properties
		csProps, exists := (*result)[model.CredentialServiceType]
		require.True(t, exists)
		assert.Equal(t, "/path/to/keystore", csProps["keystore"])
		assert.Equal(t, true, csProps["enabled"])
	})

	t.Run("handles empty input", func(t *testing.T) {
		vpaProperties := map[string]map[string]any{}

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 0)
	})

	t.Run("handles nil input", func(t *testing.T) {
		var vpaProperties map[string]map[string]any

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 0)
	})

	t.Run("handles all VPA types", func(t *testing.T) {
		vpaProperties := map[string]map[string]any{
			string(model.ConnectorType): {
				"connector_prop": "connector_value",
			},
			string(model.CredentialServiceType): {
				"creds_prop": "creds_value",
			},
			string(model.DataPlaneType): {
				"dataplane_prop": "dataplane_value",
			},
		}

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 3)

		assert.Contains(t, *result, model.ConnectorType)
		assert.Contains(t, *result, model.CredentialServiceType)
		assert.Contains(t, *result, model.DataPlaneType)

		assert.Equal(t, "connector_value", (*result)[model.ConnectorType]["connector_prop"])
		assert.Equal(t, "creds_value", (*result)[model.CredentialServiceType]["creds_prop"])
		assert.Equal(t, "dataplane_value", (*result)[model.DataPlaneType]["dataplane_prop"])
	})

	t.Run("handles custom VPA type", func(t *testing.T) {
		customVPAType := "custom.vpa.type"
		vpaProperties := map[string]map[string]any{
			customVPAType: {
				"custom_prop": "custom_value",
				"count":       42,
			},
		}

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 1)

		customType := model.VPAType(customVPAType)
		customProps, exists := (*result)[customType]
		require.True(t, exists)
		assert.Equal(t, "custom_value", customProps["custom_prop"])
		assert.Equal(t, 42, customProps["count"])
	})

	t.Run("handles empty properties for a VPA type", func(t *testing.T) {
		vpaProperties := map[string]map[string]any{
			string(model.ConnectorType): {},
			string(model.DataPlaneType): {
				"prop": "value",
			},
		}

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 2)

		connectorProps, exists := (*result)[model.ConnectorType]
		require.True(t, exists)
		assert.Len(t, connectorProps, 0)

		dataplaneProps, exists := (*result)[model.DataPlaneType]
		require.True(t, exists)
		assert.Len(t, dataplaneProps, 1)
		assert.Equal(t, "value", dataplaneProps["prop"])
	})

	t.Run("handles nil properties for a VPA type", func(t *testing.T) {
		vpaProperties := map[string]map[string]any{
			string(model.ConnectorType): nil,
		}

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 1)

		connectorProps, exists := (*result)[model.ConnectorType]
		require.True(t, exists)
		assert.Nil(t, connectorProps)
	})

	t.Run("handles complex property values", func(t *testing.T) {
		nestedMap := map[string]any{
			"nested_key": "nested_value",
			"nested_num": 123,
		}
		slice := []string{"item1", "item2", "item3"}

		vpaProperties := map[string]map[string]any{
			string(model.ConnectorType): {
				"string_prop": "string_value",
				"int_prop":    42,
				"float_prop":  3.14,
				"bool_prop":   true,
				"map_prop":    nestedMap,
				"slice_prop":  slice,
				"nil_prop":    nil,
			},
		}

		result := ToVPAMap(vpaProperties)

		require.NotNil(t, result)
		assert.Len(t, *result, 1)

		props := (*result)[model.ConnectorType]
		assert.Equal(t, "string_value", props["string_prop"])
		assert.Equal(t, 42, props["int_prop"])
		assert.Equal(t, 3.14, props["float_prop"])
		assert.Equal(t, true, props["bool_prop"])
		assert.Equal(t, nestedMap, props["map_prop"])
		assert.Equal(t, slice, props["slice_prop"])
		assert.Nil(t, props["nil_prop"])
	})

	t.Run("returns pointer to new VPAPropMap", func(t *testing.T) {
		vpaProperties := map[string]map[string]any{
			string(model.ConnectorType): {
				"prop": "value",
			},
		}

		result1 := ToVPAMap(vpaProperties)
		result2 := ToVPAMap(vpaProperties)

		require.NotNil(t, result1)
		require.NotNil(t, result2)

		// Should be different instances
		assert.NotSame(t, result1, result2)

		// But should contain same data
		assert.Equal(t, *result1, *result2)
	})

}
