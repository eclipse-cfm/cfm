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
	"context"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RequiredFieldsType struct {
	ID       string         `json:"id" validate:"required"`
	Name     string         `json:"name" validate:"required"`
	Value    int            `json:"value" validate:"required"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type NestedRequiredFieldsType struct {
	ID     string             `json:"id" validate:"required"`
	Nested RequiredFieldsType `json:"nested" validate:"required"`
	Extra  string             `json:"extra,omitempty"`
}

func TestReadValues_ReadToMap(t *testing.T) {
	ctx := context.Background()
	processingData := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": true,
		"key4": map[string]any{
			"nested": "data",
		},
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result map[string]any
	err := activityContext.ReadValues(&result)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, float64(42), result["key2"]) // JSON unmarshals numbers as float64
	assert.Equal(t, true, result["key3"])
	assert.NotNil(t, result["key4"])
}

func TestReadValues_ReadToComplexTypeWithRequiredFieldsSuccess(t *testing.T) {
	ctx := context.Background()
	processingData := map[string]any{
		"id":    "obj-123",
		"name":  "Test Object",
		"value": 100,
		"metadata": map[string]any{
			"created": "2025-01-01",
			"author":  "test",
		},
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result RequiredFieldsType
	err := activityContext.ReadValues(&result)

	require.NoError(t, err)
	assert.Equal(t, "obj-123", result.ID)
	assert.Equal(t, "Test Object", result.Name)
	assert.Equal(t, 100, result.Value)
	assert.Len(t, result.Metadata, 2)
	assert.Equal(t, "2025-01-01", result.Metadata["created"])
	assert.Equal(t, "test", result.Metadata["author"])
}

func TestReadValues_ReadToNestedComplexTypeSuccess(t *testing.T) {
	ctx := context.Background()
	processingData := map[string]any{
		"id":    "parent-1",
		"extra": "extra-data",
		"nested": map[string]any{
			"id":    "child-1",
			"name":  "Nested Object",
			"value": 50,
			"metadata": map[string]any{
				"level": "nested",
			},
		},
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result NestedRequiredFieldsType
	err := activityContext.ReadValues(&result)

	require.NoError(t, err)
	assert.Equal(t, "parent-1", result.ID)
	assert.Equal(t, "extra-data", result.Extra)
	assert.Equal(t, "child-1", result.Nested.ID)
	assert.Equal(t, "Nested Object", result.Nested.Name)
	assert.Equal(t, 50, result.Nested.Value)
	assert.Len(t, result.Nested.Metadata, 1)
	assert.Equal(t, "nested", result.Nested.Metadata["level"])
}

func TestReadValues_ReadToComplexTypeWithMissingRequiredFieldError(t *testing.T) {
	ctx := context.Background()
	nameMissing := map[string]any{
		"id": "obj-123",
		// "name" is missing
		"value": 100,
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), nameMissing, outputData)

	var result RequiredFieldsType
	err := activityContext.ReadValues(&result)

	require.Error(t, err)
}

func TestReadValues_ReadToComplexTypeWithInvalidTypeError(t *testing.T) {
	ctx := context.Background()
	processingData := map[string]any{
		"id":    "obj-123",
		"name":  "Test Object",
		"value": "not-a-number", // This is a string but we expect int
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result RequiredFieldsType
	err := activityContext.ReadValues(&result)

	// JSON unmarshal will fail when trying to unmarshal a string to an int
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal")
}

func TestReadValues_ReadToStringUnsuccessfully(t *testing.T) {
	ctx := context.Background()
	processingData := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result string
	err := activityContext.ReadValues(&result)

	// Cannot unmarshal a JSON object into a string
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal")
}

func TestReadValues_ReadToStringSliceUnsuccessfully(t *testing.T) {
	ctx := context.Background()
	processingData := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result []string
	err := activityContext.ReadValues(&result)

	// Cannot unmarshal a JSON object into a string slice
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal")
}

func TestReadValues_ReadToMapWithArrayData(t *testing.T) {
	ctx := context.Background()
	processingData := map[string]any{
		"items": []any{"item1", "item2", "item3"},
		"count": 3,
	}
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result map[string]any
	err := activityContext.ReadValues(&result)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result["items"].([]any), 3)
	assert.Equal(t, float64(3), result["count"])
}

func TestReadValues_ReadToComplexTypeWithEmptyProcessingData(t *testing.T) {
	ctx := context.Background()
	processingData := make(map[string]any) // Empty map
	outputData := make(map[string]any)
	activityContext := NewActivityContext(ctx, "orch-1", getTestActivity(), processingData, outputData)

	var result RequiredFieldsType
	err := activityContext.ReadValues(&result)

	require.Error(t, err)
}

func getTestActivity() Activity {
	return Activity{
		ID:            "test-activity",
		Type:          "test",
		Discriminator: DeployDiscriminator,
	}
}

const testVpaType model.VPAType = "cfm.test.vpa.type"
const otherVpaType model.VPAType = "cfm.test.vpa.other"

func TestVpaProperties_MissingKey(t *testing.T) {
	ctx := NewActivityContext(context.Background(), "orch-1", getTestActivity(), map[string]any{}, map[string]any{})

	_, err := ctx.VpaProperties(testVpaType)

	require.Error(t, err)
	assert.ErrorContains(t, err, model.VPAData)
}

func TestVpaProperties_NilValue(t *testing.T) {
	processingData := map[string]any{model.VPAData: nil}
	ctx := NewActivityContext(context.Background(), "orch-1", getTestActivity(), processingData, map[string]any{})

	_, err := ctx.VpaProperties(testVpaType)

	require.Error(t, err)
	assert.ErrorContains(t, err, model.VPAData)
}

func TestVpaProperties_NotASlice(t *testing.T) {
	processingData := map[string]any{model.VPAData: "not-a-slice"}
	ctx := NewActivityContext(context.Background(), "orch-1", getTestActivity(), processingData, map[string]any{})

	_, err := ctx.VpaProperties(testVpaType)

	require.Error(t, err)
	assert.ErrorContains(t, err, "not a slice")
}

func TestVpaProperties_EmptySlice(t *testing.T) {
	processingData := map[string]any{model.VPAData: []any{}}
	ctx := NewActivityContext(context.Background(), "orch-1", getTestActivity(), processingData, map[string]any{})

	_, err := ctx.VpaProperties(testVpaType)

	require.Error(t, err)
	assert.ErrorContains(t, err, testVpaType.String())
}

func TestVpaProperties_NoMatchingType(t *testing.T) {
	processingData := map[string]any{
		model.VPAData: []any{
			map[string]any{"vpaType": otherVpaType.String()},
		},
	}
	ctx := NewActivityContext(context.Background(), "orch-1", getTestActivity(), processingData, map[string]any{})

	_, err := ctx.VpaProperties(testVpaType)

	require.Error(t, err)
	assert.ErrorContains(t, err, testVpaType.String())
}

func TestVpaProperties_MatchingTypeWithProperties(t *testing.T) {
	props := map[string]any{"region": "eu-west", "tier": "standard"}
	processingData := map[string]any{
		model.VPAData: []any{
			map[string]any{
				"vpaType":    testVpaType.String(),
				"properties": props,
			},
		},
	}
	ctx := NewActivityContext(context.Background(), "orch-1", getTestActivity(), processingData, map[string]any{})

	result, err := ctx.VpaProperties(testVpaType)

	require.NoError(t, err)
	assert.Equal(t, props, result)
}

func TestVpaProperties_MatchingTypeWithoutProperties(t *testing.T) {
	processingData := map[string]any{
		model.VPAData: []any{
			map[string]any{"vpaType": testVpaType.String()},
		},
	}
	ctx := NewActivityContext(context.Background(), "orch-1", getTestActivity(), processingData, map[string]any{})

	result, err := ctx.VpaProperties(testVpaType)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestActivityContext_Delete(t *testing.T) {
	activity := Activity{ID: "test-activity"}
	activityContext := NewActivityContext(context.TODO(), "test-oid", activity, map[string]any{}, map[string]any{})

	// Set a value
	activityContext.SetValue("key", "value")

	// Verify it exists
	_, exists := activityContext.Value("key")
	assert.True(t, exists)

	// Delete the key
	activityContext.Delete("key")

	// Verify it's deleted
	_, exists = activityContext.Value("key")
	assert.False(t, exists)
}
