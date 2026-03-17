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

package memorystore

import (
	"context"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefinitionStore(t *testing.T) {
	definitionStore := NewDefinitionStore()

	assert.NotNil(t, definitionStore)
	assert.NotNil(t, definitionStore.orchestrationDefinitions)
	assert.NotNil(t, definitionStore.activityDefinitions)
	assert.Equal(t, 0, len(definitionStore.orchestrationDefinitions))
	assert.Equal(t, 0, len(definitionStore.activityDefinitions))
}

func TestDefinitionStore_OrchestrationDefinition_StoreAndFind(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var oType model.OrchestrationType = "test-orchestration-1"
	definition := &api.OrchestrationDefinition{
		Type: oType,
	}

	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, definition)

	result, err := definitionStore.FindOrchestrationDefinition(ctx, oType)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, definition.Type, result.Type)

	// Verify it's a copy (different memory address)
	assert.NotSame(t, definition, result)
}

func TestDefinitionStore_OrchestrationDefinition_FindNotFound(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	result, err := definitionStore.FindOrchestrationDefinition(ctx, "non-existent")

	assert.Error(t, err)
	assert.Equal(t, types.ErrNotFound, err)
	assert.Nil(t, result)
}

func TestDefinitionStore_ActivityDefinition_StoreAndFind(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var activityType api.ActivityType = "test-activity-1"
	definition := &api.ActivityDefinition{
		Type:        activityType,
		Description: "Test activity",
	}

	_, _ = definitionStore.StoreActivityDefinition(ctx, definition)

	result, err := definitionStore.FindActivityDefinition(ctx, activityType)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, definition.Type, result.Type)
	assert.Equal(t, definition.Description, result.Description)

	// Verify it's a copy (different memory address)
	assert.NotSame(t, definition, result)
}

func TestDefinitionStore_ActivityDefinition_FindNotFound(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	result, err := definitionStore.FindActivityDefinition(ctx, "non-existent")

	assert.Error(t, err)
	assert.Equal(t, types.ErrNotFound, err)
	assert.Nil(t, result)
}

func TestDefinitionStore_OrchestrationDefinition_Delete(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var oType model.OrchestrationType = "test-orchestration-1"
	definition := &api.OrchestrationDefinition{Type: oType}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, definition)

	_, err := definitionStore.FindOrchestrationDefinition(ctx, oType)
	assert.NoError(t, err)

	deleted, err := definitionStore.DeleteOrchestrationDefinition(ctx, oType)
	assert.Nil(t, err)
	assert.True(t, deleted)

	_, err = definitionStore.FindOrchestrationDefinition(ctx, oType)
	assert.Equal(t, types.ErrNotFound, err)

	deleted, err = definitionStore.DeleteOrchestrationDefinition(ctx, oType)
	assert.Nil(t, err)
	assert.False(t, deleted)
}

func TestDefinitionStore_ActivityDefinition_Delete(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var activityType api.ActivityType = "test-activity-1"
	definition := &api.ActivityDefinition{Type: activityType}
	_, _ = definitionStore.StoreActivityDefinition(ctx, definition)

	_, err := definitionStore.FindActivityDefinition(ctx, activityType)
	assert.NoError(t, err)

	deleted, err := definitionStore.DeleteActivityDefinition(ctx, activityType)
	assert.Nil(t, err)
	assert.True(t, deleted)

	_, err = definitionStore.FindActivityDefinition(ctx, activityType)
	assert.Equal(t, types.ErrNotFound, err)

	deleted, err = definitionStore.DeleteActivityDefinition(ctx, activityType)
	assert.Nil(t, err)
	assert.False(t, deleted)
}

func TestDefinitionStore_DataIsolation(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var originalType model.OrchestrationType = "original-type"
	originalDef := &api.OrchestrationDefinition{
		Type: originalType,
	}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, originalDef)

	originalDef.Type = "modified-type"

	retrievedDef, err := definitionStore.FindOrchestrationDefinition(ctx, originalType)
	require.NoError(t, err)

	assert.Equal(t, originalType, retrievedDef.Type)
	assert.NotEqual(t, originalDef.Type, retrievedDef.Type)

	retrievedDef.Type = "retrieved-modified"

	retrievedDef2, err := definitionStore.FindOrchestrationDefinition(ctx, originalType)
	require.NoError(t, err)
	assert.Equal(t, originalType, retrievedDef2.Type)
}

func TestDefinitionStore_StoreOverwrite(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var dType model.OrchestrationType = "test-orchestration"

	// Store first definition
	definition1 := &api.OrchestrationDefinition{
		Type: dType,
	}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, definition1)

	// Store second definition with same ID (overwrite)
	definition2 := &api.OrchestrationDefinition{
		Type: dType,
	}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, definition2)

	// Verify the second definition is stored
	result, err := definitionStore.FindOrchestrationDefinition(ctx, dType)
	require.NoError(t, err)
	assert.Equal(t, dType, result.Type)

	// Verify only one definition exists
	defintions, err := definitionStore.ListOrchestrationDefinitions(ctx)
	assert.Equal(t, 1, len(defintions))
}

func TestDefinitionStore_ListOrchestrationDefinitions(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	// Test empty store
	definitions, err := definitionStore.ListOrchestrationDefinitions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(definitions))

	// Add test data
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, &api.OrchestrationDefinition{Type: "type1"})

	definitions, err = definitionStore.ListOrchestrationDefinitions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(definitions))

}

func TestDefinitionStore_ListOrchestrationDefinitions_DataIsolation(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var originalType model.OrchestrationType = "original"
	originalDef := &api.OrchestrationDefinition{Type: originalType}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, originalDef)

	definitions, err := definitionStore.ListOrchestrationDefinitions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(definitions))

	// Modify the original definition
	originalDef.Type = "modified"

	// Verify returned definition is not affected
	assert.Equal(t, originalType, definitions[0].Type)

	// Modify the returned definition
	definitions[0].Type = "returned-modified"

	// Verify stored definition is not affected
	storedDef, err := definitionStore.FindOrchestrationDefinition(ctx, originalType)
	assert.NoError(t, err)
	assert.Equal(t, originalType, storedDef.Type)
}

func TestDefinitionStore_ListActivityDefinitions(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	// Test empty store
	definitions, err := definitionStore.ListActivityDefinitions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(definitions))

	_, _ = definitionStore.StoreActivityDefinition(ctx, &api.ActivityDefinition{Type: "type1", Description: "desc1"})

	definitions, err = definitionStore.ListActivityDefinitions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(definitions))
}

func TestDefinitionStore_ListActivityDefinitions_DataIsolation(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var originalType api.ActivityType = "original"
	originalDef := &api.ActivityDefinition{Type: originalType, Description: "desc1"}
	_, _ = definitionStore.StoreActivityDefinition(ctx, originalDef)

	definitions, err := definitionStore.ListActivityDefinitions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(definitions))

	// Modify the original definition
	originalDef.Type = "modified"

	// Verify returned definition is not affected
	assert.Equal(t, originalType, definitions[0].Type)

	// Modify the returned definition
	definitions[0].Type = "returned-modified"

	// Verify stored definition is not affected
	storedDef, err := definitionStore.FindActivityDefinition(ctx, originalType)
	assert.NoError(t, err)
	assert.Equal(t, originalType, storedDef.Type)
}

func TestDefinitionStore_ExistsOrchestrationDefinition_Found(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var oType model.OrchestrationType = "test-orchestration-exists"
	definition := &api.OrchestrationDefinition{Type: oType}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, definition)

	exists, err := definitionStore.ExistsOrchestrationDefinition(ctx, oType)

	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestDefinitionStore_ExistsOrchestrationDefinition_NotFound(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	exists, err := definitionStore.ExistsOrchestrationDefinition(ctx, "non-existent-orchestration")

	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestDefinitionStore_ExistsActivityDefinition_Found(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	var activityType api.ActivityType = "test-activity-exists"
	definition := &api.ActivityDefinition{Type: activityType}
	_, _ = definitionStore.StoreActivityDefinition(ctx, definition)

	exists, err := definitionStore.ExistsActivityDefinition(ctx, activityType)

	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestDefinitionStore_ExistsActivityDefinition_NotFound(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	exists, err := definitionStore.ExistsActivityDefinition(ctx, "non-existent-activity")

	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestDefinitionStore_ActivityDefinitionReferenced_Found(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	// Store an activity definition
	activityDef := &api.ActivityDefinition{
		Type:        "deploy-activity",
		Description: "Deploy resources",
	}
	_, _ = definitionStore.StoreActivityDefinition(ctx, activityDef)

	orchestrationDef := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("deploy-orchestration"),
		Active: true,
		Activities: []api.Activity{
			{
				ID:   "activity-1",
				Type: "deploy-activity",
			},
			{ // Include a second call to ensure the orchestration type is returned only once
				ID:   "activity-1",
				Type: "deploy-activity",
			},
		},
	}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, orchestrationDef)

	referenced, err := definitionStore.ActivityDefinitionReferences(ctx, "deploy-activity")

	assert.NoError(t, err)
	assert.ElementsMatch(t, referenced, []string{"deploy-orchestration"})
}

func TestDefinitionStore_ActivityDefinitionReferenced_NotFound(t *testing.T) {
	definitionStore := NewDefinitionStore()
	ctx := context.Background()

	orchestrationDef := &api.OrchestrationDefinition{
		Type:       model.OrchestrationType("empty-orchestration"),
		Active:     true,
		Activities: []api.Activity{},
	}
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, orchestrationDef)

	// Verify activity is not referenced
	referenced, err := definitionStore.ActivityDefinitionReferences(ctx, "non-existent-activity")

	assert.NoError(t, err)
	assert.Empty(t, referenced)
}
