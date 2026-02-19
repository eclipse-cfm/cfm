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

package v1alpha1

import (
	"testing"
	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToAPIActivityDefinition(t *testing.T) {
	tests := []struct {
		name       string
		definition *ActivityDefinitionDto
		expected   *api.ActivityDefinition
	}{
		{
			name: "complete activity definition",
			definition: &ActivityDefinitionDto{
				Type:         "http-request",
				Description:  "Makes HTTP requests",
				InputSchema:  map[string]any{"url": "string"},
				OutputSchema: map[string]any{"response": "object"},
			},
			expected: &api.ActivityDefinition{
				Type:         api.ActivityType("http-request"),
				Description:  "Makes HTTP requests",
				InputSchema:  map[string]any{"url": "string"},
				OutputSchema: map[string]any{"response": "object"},
			},
		},
		{
			name: "minimal activity definition",
			definition: &ActivityDefinitionDto{
				Type: "basic-task",
			},
			expected: &api.ActivityDefinition{
				Type: api.ActivityType("basic-task"),
			},
		},
		{
			name:       "empty activity definition",
			definition: &ActivityDefinitionDto{},
			expected: &api.ActivityDefinition{
				Type: api.ActivityType(""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToActivityDefinition(tt.definition)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToAPIActivityDefinition_NilInput(t *testing.T) {
	// Test that the function handles nil input gracefully
	assert.NotPanics(t, func() {
		result := ToActivityDefinition(nil)
		assert.Empty(t, result.Type)
		assert.Empty(t, result.Description)
		assert.Nil(t, result.InputSchema)
		assert.Nil(t, result.OutputSchema)
	})
}

func TestToOrchestrationDefinition(t *testing.T) {
	tests := []struct {
		name                    string
		orchestrationDefinition *OrchestrationTemplate
		expected                []*api.OrchestrationDefinition
	}{
		{
			name: "complete orchestration definition",
			orchestrationDefinition: &OrchestrationTemplate{
				Description: "Test",
				Schema:      map[string]any{"version": "v1"},
				Activities: map[string][]ActivityDto{
					"foo.bar.kubernetes": {
						{
							ID:   "activity-1",
							Type: "http-request",
							Inputs: []MappingEntry{
								{Source: "input.url", Target: "request.url"},
								{Source: "input.method", Target: "request.method"},
							},
							DependsOn: []string{"activity-0"},
						},
					},
					DefaultActivityDiscriminator: {{
						ID:        "activity-2",
						Type:      "data-transform",
						Inputs:    []MappingEntry{},
						DependsOn: []string{"activity-1"},
					}},
				},
			},
			expected: []*api.OrchestrationDefinition{
				{
					Type:        model.OrchestrationType("foo.bar.kubernetes"),
					Description: "Test",
					Active:      true,
					Schema:      map[string]any{"version": "v1"},
					Activities: []api.Activity{
						{
							ID:            "activity-1",
							Type:          api.ActivityType("http-request"),
							Discriminator: "foo.bar.kubernetes",
							Inputs: []api.MappingEntry{
								{Source: "input.url", Target: "request.url"},
								{Source: "input.method", Target: "request.method"},
							},
							DependsOn: []string{"activity-0"},
						},
					},
				},
				{
					Type:        model.OrchestrationType(DefaultActivityDiscriminator),
					Description: "Test",
					Active:      true,
					Schema:      map[string]any{"version": "v1"},
					Activities: []api.Activity{
						{
							ID:            "activity-2",
							Discriminator: DefaultActivityDiscriminator,
							Type:          api.ActivityType("data-transform"),
							Inputs:        []api.MappingEntry{},
							DependsOn:     []string{"activity-1"},
						},
					},
				},
			},
		},
		{
			name: "minimal orchestration definition",
			orchestrationDefinition: &OrchestrationTemplate{
				Activities: map[string][]ActivityDto{},
			},
			expected: []*api.OrchestrationDefinition{
				{
					Active:     true,
					Activities: []api.Activity{},
				},
			},
		},
		{
			name:                    "empty orchestration definition",
			orchestrationDefinition: &OrchestrationTemplate{},
			expected:                []*api.OrchestrationDefinition{},
		},
		{
			name: "single activity without dependencies",
			orchestrationDefinition: &OrchestrationTemplate{
				Activities: map[string][]ActivityDto{
					"local": {{
						ID:   "standalone-activity",
						Type: "file-processor",
						Inputs: []MappingEntry{
							{Source: "file.path", Target: "processor.input"},
						},
					}},
				},
			},
			expected: []*api.OrchestrationDefinition{{
				Type:   model.OrchestrationType("local"),
				Active: true,
				Activities: []api.Activity{
					{
						ID:            "standalone-activity",
						Type:          api.ActivityType("file-processor"),
						Discriminator: "local",
						Inputs: []api.MappingEntry{
							{Source: "file.path", Target: "processor.input"},
						},
					},
				},
			},
			},
		}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToOrchestrationDefinition(tt.orchestrationDefinition)
			assert.Equal(t, tt.expected, result)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestToAPIOrchestrationDefinition_NilInput(t *testing.T) {
	// Test that the function handles nil input gracefully
	assert.NotPanics(t, func() {
		result := ToOrchestrationDefinition(nil)
		assert.Empty(t, result)
	})
}

func TestToAPIMappingEntries(t *testing.T) {
	tests := []struct {
		name     string
		entries  []MappingEntry
		expected []api.MappingEntry
	}{
		{
			name: "multiple mapping entries",
			entries: []MappingEntry{
				{Source: "input.name", Target: "output.fullName"},
				{Source: "input.age", Target: "output.yearsOld"},
				{Source: "input.email", Target: "output.contactEmail"},
			},
			expected: []api.MappingEntry{
				{Source: "input.name", Target: "output.fullName"},
				{Source: "input.age", Target: "output.yearsOld"},
				{Source: "input.email", Target: "output.contactEmail"},
			},
		},
		{
			name: "single mapping entry",
			entries: []MappingEntry{
				{Source: "data.value", Target: "result.processed"},
			},
			expected: []api.MappingEntry{
				{Source: "data.value", Target: "result.processed"},
			},
		},
		{
			name:     "empty mapping entries",
			entries:  []MappingEntry{},
			expected: []api.MappingEntry{},
		},
		{
			name: "mapping entries with empty strings",
			entries: []MappingEntry{
				{Source: "", Target: ""},
				{Source: "valid.source", Target: ""},
				{Source: "", Target: "valid.target"},
			},
			expected: []api.MappingEntry{
				{Source: "", Target: ""},
				{Source: "valid.source", Target: ""},
				{Source: "", Target: "valid.target"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToAPIMappingEntries(tt.entries)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToAPIMappingEntries_NilInput(t *testing.T) {
	result := ToAPIMappingEntries(nil)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestToOrchestrationEntry_VerifiesInputs(t *testing.T) {
	testTime := time.Now()
	input := api.OrchestrationEntry{
		ID:                "test-id-123",
		CorrelationID:     "corr-id-456",
		State:             5,
		StateTimestamp:    testTime,
		CreatedTimestamp:  testTime.Add(-time.Hour),
		OrchestrationType: model.OrchestrationType("TestType"),
	}

	result := ToOrchestrationEntry(&input)

	assert.Equal(t, input.ID, result.ID)
	assert.Equal(t, input.CorrelationID, result.CorrelationID)
	assert.Equal(t, int(input.State), result.State)
	assert.Equal(t, input.StateTimestamp, result.StateTimestamp)
	assert.Equal(t, input.CreatedTimestamp, result.CreatedTimestamp)
	assert.Equal(t, input.OrchestrationType, result.OrchestrationType)
}

func TestToOrchestration(t *testing.T) {
	now := time.Now()
	apiOrchestration := &api.Orchestration{
		ID:                "test-orch-1",
		CorrelationID:     "corr-123",
		State:             api.OrchestrationStateRunning,
		StateTimestamp:    now,
		CreatedTimestamp:  now.Add(-1 * time.Hour),
		OrchestrationType: "test-type",
		ProcessingData:    map[string]any{"key1": "value1"},
		OutputData:        map[string]any{"key2": "value2"},
		Completed:         map[string]struct{}{"activity1": {}},
		Steps: []api.OrchestrationStep{
			{
				Activities: []api.Activity{
					{
						ID:            "activity-1",
						Type:          "test.activity",
						Discriminator: "test-disc",
						Inputs: []api.MappingEntry{
							{Source: "src1", Target: "tgt1"},
						},
						DependsOn: []string{"activity-0"},
					},
				},
			},
		},
	}

	result := ToOrchestration(apiOrchestration)

	assert.Equal(t, "test-orch-1", result.ID)
	assert.Equal(t, "corr-123", result.CorrelationID)
	assert.Equal(t, int(api.OrchestrationStateRunning), result.State)
	assert.Equal(t, now, result.StateTimestamp)
	assert.Equal(t, now.Add(-1*time.Hour), result.CreatedTimestamp)
	assert.Equal(t, model.OrchestrationType("test-type"), result.OrchestrationType)
	assert.Equal(t, map[string]any{"key1": "value1"}, result.ProcessingData)
	assert.Equal(t, map[string]any{"key2": "value2"}, result.OutputData)
	assert.Equal(t, 1, len(result.Steps))
	assert.Equal(t, 1, len(result.Steps[0].Activities))
	assert.Equal(t, "activity-1", result.Steps[0].Activities[0].ID)
	assert.Equal(t, "test.activity", result.Steps[0].Activities[0].Type)
	assert.Equal(t, 1, len(result.Steps[0].Activities[0].Inputs))
	assert.Equal(t, "src1", result.Steps[0].Activities[0].Inputs[0].Source)
}

func TestToActivityDefinition_WithValidDefinition(t *testing.T) {
	// Arrange
	inputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field1": map[string]any{"type": "string"},
		},
	}
	outputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]any{"type": "boolean"},
		},
	}

	apiDef := &api.ActivityDefinition{
		Type:         api.ActivityType("HTTP_REQUEST"),
		Description:  "Makes an HTTP request",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}

	// Act
	result := ToActivityDefinitionDto(apiDef)

	// Assert
	require.NotNil(t, result)
	assert.Equal(t, "HTTP_REQUEST", result.Type)
	assert.Equal(t, "Makes an HTTP request", result.Description)
	assert.Equal(t, inputSchema, result.InputSchema)
	assert.Equal(t, outputSchema, result.OutputSchema)
}

func TestToActivityDefinition_WithNilDefinition(t *testing.T) {
	// Act
	result := ToActivityDefinitionDto(nil)

	// Assert
	require.NotNil(t, result)
	assert.Equal(t, "", result.Type)
	assert.Equal(t, "", result.Description)
	assert.Nil(t, result.InputSchema)
	assert.Nil(t, result.OutputSchema)
}

func TestToActivityDefinition_WithEmptySchemas(t *testing.T) {
	// Arrange
	apiDef := &api.ActivityDefinition{
		Type:         api.ActivityType("CUSTOM"),
		Description:  "Custom activity",
		InputSchema:  make(map[string]any),
		OutputSchema: make(map[string]any),
	}

	// Act
	result := ToActivityDefinitionDto(apiDef)

	// Assert
	require.NotNil(t, result)
	assert.Equal(t, "CUSTOM", result.Type)
	assert.Equal(t, "Custom activity", result.Description)
	assert.NotNil(t, result.InputSchema)
	assert.NotNil(t, result.OutputSchema)
	assert.Equal(t, 0, len(result.InputSchema))
	assert.Equal(t, 0, len(result.OutputSchema))
}

func TestToActivityDefinition_WithComplexSchemas(t *testing.T) {
	// Arrange
	inputSchema := map[string]any{
		"type":     "object",
		"required": []string{"url", "method"},
		"properties": map[string]any{
			"url":    map[string]any{"type": "string", "format": "uri"},
			"method": map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT"}},
			"headers": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
			},
		},
	}
	outputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"statusCode": map[string]any{"type": "integer"},
			"body":       map[string]any{"type": "string"},
		},
	}

	apiDef := &api.ActivityDefinition{
		Type:         api.ActivityType("API_CALL"),
		Description:  "Calls an external API",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
	}

	// Act
	result := ToActivityDefinitionDto(apiDef)

	// Assert
	require.NotNil(t, result)
	assert.Equal(t, "API_CALL", result.Type)
	assert.Equal(t, inputSchema, result.InputSchema)
	assert.Equal(t, outputSchema, result.OutputSchema)
}

//func TestToOrchestrationDefinition(t *testing.T) {
//	tests := []struct {
//		name       string
//		definition *api.OrchestrationDefinition
//		expected   *OrchestrationDefinitionDto
//	}{
//		{
//			name: "complete orchestration definition",
//			definition: &api.OrchestrationDefinition{
//				Type:        model.OrchestrationType("kubernetes"),
//				Description: "Deploy Kubernetes VPA",
//				Active:      true,
//				Schema:      map[string]any{"version": "v1", "kind": "Deployment"},
//				Activities: []api.Activity{
//					{
//						ID:            "activity-1",
//						Type:          api.ActivityType("http-request"),
//						Discriminator: api.Discriminator("deploy"),
//						Inputs: []api.MappingEntry{
//							{Source: "input.url", Target: "request.url"},
//							{Source: "input.method", Target: "request.method"},
//						},
//						DependsOn: []string{"activity-0"},
//					},
//					{
//						ID:        "activity-2",
//						Type:      api.ActivityType("data-transform"),
//						Inputs:    []api.MappingEntry{},
//						DependsOn: []string{"activity-1"},
//					},
//				},
//			},
//			expected: &OrchestrationDefinitionDto{
//				Type:        "kubernetes",
//				Description: "Deploy Kubernetes VPA",
//				Schema:      map[string]any{"version": "v1", "kind": "Deployment"},
//				Activities: []ActivityDto{
//					{
//						ID:            "activity-1",
//						Type:          "http-request",
//						Discriminator: "deploy",
//						Inputs: []MappingEntry{
//							{Source: "input.url", Target: "request.url"},
//							{Source: "input.method", Target: "request.method"},
//						},
//						DependsOn: []string{"activity-0"},
//					},
//					{
//						ID:        "activity-2",
//						Type:      "data-transform",
//						Inputs:    []MappingEntry{},
//						DependsOn: []string{"activity-1"},
//					},
//				},
//			},
//		},
//		{
//			name: "minimal orchestration definition",
//			definition: &api.OrchestrationDefinition{
//				Type:       model.OrchestrationType("docker"),
//				Active:     true,
//				Activities: []api.Activity{},
//			},
//			expected: &OrchestrationDefinitionDto{
//				Type:       "docker",
//				Activities: []ActivityDto{},
//			},
//		},
//		{
//			name: "empty orchestration definition",
//			definition: &api.OrchestrationDefinition{
//				Type:       model.OrchestrationType(""),
//				Active:     true,
//				Activities: []api.Activity{},
//			},
//			expected: &OrchestrationDefinitionDto{
//				Type:       "",
//				Activities: []ActivityDto{},
//			},
//		},
//		{
//			name: "single activity without dependencies",
//			definition: &api.OrchestrationDefinition{
//				Type:   model.OrchestrationType("local"),
//				Active: true,
//				Activities: []api.Activity{
//					{
//						ID:   "standalone-activity",
//						Type: api.ActivityType("file-processor"),
//						Inputs: []api.MappingEntry{
//							{Source: "file.path", Target: "processor.input"},
//						},
//					},
//				},
//			},
//			expected: &OrchestrationDefinitionDto{
//				Type: "local",
//				Activities: []ActivityDto{
//					{
//						ID:   "standalone-activity",
//						Type: "file-processor",
//						Inputs: []MappingEntry{
//							{Source: "file.path", Target: "processor.input"},
//						},
//					},
//				},
//			},
//		},
//		{
//			name: "multiple activities with complex dependencies",
//			definition: &api.OrchestrationDefinition{
//				Type:        model.OrchestrationType("cfm.orchestration.vpa.deploy"),
//				Description: "VPA Deployment Orchestration",
//				Active:      true,
//				Schema: map[string]any{
//					"orchestrationVersion": "1.0",
//					"timeoutSeconds":       3600,
//				},
//				Activities: []api.Activity{
//					{
//						ID:            "validate",
//						Type:          api.ActivityType("validation"),
//						Discriminator: api.Discriminator("schema-check"),
//						Inputs: []api.MappingEntry{
//							{Source: "manifest.payload", Target: "validator.input"},
//						},
//						DependsOn: []string{},
//					},
//					{
//						ID:            "provision",
//						Type:          api.ActivityType("provisioning"),
//						Discriminator: api.Discriminator("vpa-provision"),
//						Inputs: []api.MappingEntry{
//							{Source: "validated.data", Target: "provisioner.config"},
//							{Source: "credentials.service", Target: "provisioner.auth"},
//						},
//						DependsOn: []string{"validate"},
//					},
//					{
//						ID:            "deploy",
//						Type:          api.ActivityType("deployment"),
//						Discriminator: api.Discriminator("k8s-deploy"),
//						Inputs: []api.MappingEntry{
//							{Source: "provisioned.vpa", Target: "deployer.manifest"},
//						},
//						DependsOn: []string{"provision"},
//					},
//				},
//			},
//			expected: &OrchestrationDefinitionDto{
//				Type:        "cfm.orchestration.vpa.deploy",
//				Description: "VPA Deployment Orchestration",
//				Schema: map[string]any{
//					"orchestrationVersion": "1.0",
//					"timeoutSeconds":       3600,
//				},
//				Activities: []ActivityDto{
//					{
//						ID:            "validate",
//						Type:          "validation",
//						Discriminator: "schema-check",
//						Inputs: []MappingEntry{
//							{Source: "manifest.payload", Target: "validator.input"},
//						},
//						DependsOn: []string{},
//					},
//					{
//						ID:            "provision",
//						Type:          "provisioning",
//						Discriminator: "vpa-provision",
//						Inputs: []MappingEntry{
//							{Source: "validated.data", Target: "provisioner.config"},
//							{Source: "credentials.service", Target: "provisioner.auth"},
//						},
//						DependsOn: []string{"validate"},
//					},
//					{
//						ID:            "deploy",
//						Type:          "deployment",
//						Discriminator: "k8s-deploy",
//						Inputs: []MappingEntry{
//							{Source: "provisioned.vpa", Target: "deployer.manifest"},
//						},
//						DependsOn: []string{"provision"},
//					},
//				},
//			},
//		},
//		{
//			name: "orchestration with empty schema",
//			definition: &api.OrchestrationDefinition{
//				Type:       model.OrchestrationType("test"),
//				Schema:     make(map[string]any),
//				Active:     false,
//				Activities: []api.Activity{},
//			},
//			expected: &OrchestrationDefinitionDto{
//				Type:       "test",
//				Schema:     make(map[string]any),
//				Activities: []ActivityDto{},
//			},
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			result := ToOrchestrationDefinitionDto(tt.definition)
//			require.NotNil(t, result)
//			assert.Equal(t, tt.expected, result)
//		})
//	}
//}

//func TestToOrchestrationDefinition_NilInput(t *testing.T) {
//	// Test that the function handles nil input gracefully
//	assert.NotPanics(t, func() {
//		result := ToOrchestrationDefinitionDto(nil)
//		require.NotNil(t, result)
//		assert.Empty(t, result.Type)
//		assert.Empty(t, result.Description)
//		assert.Nil(t, result.Schema)
//		assert.Len(t, result.Activities, 0)
//	})
//}

//func TestToOrchestrationDefinition_FieldMapping(t *testing.T) {
//	// Test that all fields are correctly mapped
//	definition := &api.OrchestrationDefinition{
//		Type:        model.OrchestrationType("test-type"),
//		Description: "Test Description",
//		Active:      true,
//		Schema:      map[string]any{"key": "value"},
//		Activities: []api.Activity{
//			{
//				ID:            "test-activity",
//				Type:          api.ActivityType("test"),
//				Discriminator: api.Discriminator("discriminator"),
//				Inputs: []api.MappingEntry{
//					{Source: "src", Target: "tgt"},
//				},
//				DependsOn: []string{"dep1", "dep2"},
//			},
//		},
//	}
//
//	result := ToOrchestrationDefinitionDto(definition)
//
//	// Verify each field
//	assert.Equal(t, "test-type", result.Type)
//	assert.Equal(t, "Test Description", result.Description)
//	assert.Equal(t, map[string]any{"key": "value"}, result.Schema)
//	require.Len(t, result.Activities, 1)
//
//	activity := result.Activities[0]
//	assert.Equal(t, "test-activity", activity.ID)
//	assert.Equal(t, "test", activity.Type)
//	assert.Equal(t, "discriminator", activity.Discriminator)
//	require.Len(t, activity.Inputs, 1)
//	assert.Equal(t, "src", activity.Inputs[0].Source)
//	assert.Equal(t, "tgt", activity.Inputs[0].Target)
//	assert.Equal(t, []string{"dep1", "dep2"}, activity.DependsOn)
//}

//func TestToOrchestrationDefinition_PreservesActivityOrder(t *testing.T) {
//	// Test that activities maintain their order
//	definition := &api.OrchestrationDefinition{
//		Type:   model.OrchestrationType("order-test"),
//		Active: true,
//		Activities: []api.Activity{
//			{ID: "first", Type: api.ActivityType("type1")},
//			{ID: "second", Type: api.ActivityType("type2")},
//			{ID: "third", Type: api.ActivityType("type3")},
//		},
//	}
//
//	result := ToOrchestrationDefinitionDto(definition)
//
//	require.Len(t, result.Activities, 3)
//	assert.Equal(t, "first", result.Activities[0].ID)
//	assert.Equal(t, "second", result.Activities[1].ID)
//	assert.Equal(t, "third", result.Activities[2].ID)
//}

//func TestToOrchestrationDefinition_ActivityWithEmptyInputs(t *testing.T) {
//	// Test activity with nil and empty inputs
//	definition := &api.OrchestrationDefinition{
//		Type:   model.OrchestrationType("test"),
//		Active: true,
//		Activities: []api.Activity{
//			{
//				ID:     "activity1",
//				Type:   api.ActivityType("test"),
//				Inputs: nil,
//			},
//			{
//				ID:     "activity2",
//				Type:   api.ActivityType("test"),
//				Inputs: []api.MappingEntry{},
//			},
//		},
//	}
//
//	result := ToOrchestrationDefinitionDto(definition)
//
//	require.Len(t, result.Activities, 2)
//	assert.Len(t, result.Activities[0].Inputs, 0)
//	assert.Len(t, result.Activities[1].Inputs, 0)
//}

//func TestToOrchestrationDefinition_ActivityWithNoDependencies(t *testing.T) {
//	// Test activity with nil and empty DependsOn
//	definition := &api.OrchestrationDefinition{
//		Type:   model.OrchestrationType("test"),
//		Active: true,
//		Activities: []api.Activity{
//			{
//				ID:        "activity1",
//				Type:      api.ActivityType("test"),
//				DependsOn: nil,
//			},
//			{
//				ID:        "activity2",
//				Type:      api.ActivityType("test"),
//				DependsOn: []string{},
//			},
//		},
//	}
//
//	result := ToOrchestrationDefinitionDto(definition)
//
//	require.Len(t, result.Activities, 2)
//	assert.Len(t, result.Activities[0].DependsOn, 0)
//	assert.Len(t, result.Activities[1].DependsOn, 0)
//}
