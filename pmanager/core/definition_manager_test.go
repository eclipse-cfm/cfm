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

package core

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/google/uuid"
	memorystore2 "github.com/metaform/connector-fabric-manager/common/memorystore"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	cstore "github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/metaform/connector-fabric-manager/pmanager/memorystore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrchestrationDefinition_Success(t *testing.T) {

	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	// Create required activity definition first
	activityDef := &api.ActivityDefinition{
		Type:         "test-activity",
		Description:  "Test activity",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}
	_, err := store.StoreActivityDefinition(context.Background(), activityDef)
	require.NoError(t, err, "Failed to store activity definition")

	// Create an orchestration definition that references the activity
	orchestrationDef := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("test-orchestration"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "activity-1",
				Type:      "test-activity",
				Inputs:    []api.MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
		},
	}

	result, err := manager.CreateOrchestrationDefinition(ctx, orchestrationDef)

	require.NoError(t, err, "CreateOrchestrationDefinition should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.Equal(t, orchestrationDef.Type, result.Type, "Orchestration type should match")
	assert.Equal(t, orchestrationDef.Active, result.Active, "Active flag should match")
	assert.Equal(t, len(orchestrationDef.Activities), len(result.Activities), "Activities count should match")
}

func TestCreateOrchestrationDefinition_MissingActivityDefinition(t *testing.T) {

	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	// Create an orchestration definition that references a non-existent activity
	orchestrationDef := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("test-orchestration"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "activity-1",
				Type:      "non-existent-activity", // This activity definition doesn't exist
				Inputs:    []api.MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
		},
	}

	result, err := manager.CreateOrchestrationDefinition(ctx, orchestrationDef)

	require.Error(t, err, "CreateOrchestrationDefinition should fail when activity definition is missing")
	assert.Nil(t, result, "Result should be nil on error")

	// Verify the error is a client error about the missing activity
	var clientErr types.ClientError
	require.True(t, errors.As(err, &clientErr), "Error should be a ClientError")
	assert.Contains(t, err.Error(), "activity type 'non-existent-activity' not found",
		"Error message should mention the missing activity type")
}

func TestCreateOrchestrationDefinition_MultipleMissingActivityDefinitions(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	// Create one valid activity definition
	activityDef := &api.ActivityDefinition{
		Type:         "valid-activity",
		Description:  "Valid activity",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}
	_, err := store.StoreActivityDefinition(context.Background(), activityDef)
	require.NoError(t, err, "Failed to store valid activity definition")

	// Create orchestration definition with mix of valid and invalid activity references
	orchestrationDef := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("mixed-orchestration"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "activity-1",
				Type:      "valid-activity", // This exists
				Inputs:    []api.MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
			{
				ID:        "activity-2",
				Type:      "missing-activity-1", // This doesn't exist
				Inputs:    []api.MappingEntry{{Source: "input2", Target: "target2"}},
				DependsOn: []string{"activity-1"},
			},
			{
				ID:        "activity-3",
				Type:      "missing-activity-2", // This also doesn't exist
				Inputs:    []api.MappingEntry{{Source: "input3", Target: "target3"}},
				DependsOn: []string{"activity-1"},
			},
		},
	}

	result, err := manager.CreateOrchestrationDefinition(ctx, orchestrationDef)

	require.Error(t, err, "CreateOrchestrationDefinition should fail when multiple activity definitions are missing")
	assert.Nil(t, result, "Result should be nil on error")

	// Verify the error mentions both missing activities
	assert.Contains(t, err.Error(), "missing-activity-1", "Error should mention first missing activity")
	assert.Contains(t, err.Error(), "missing-activity-2", "Error should mention second missing activity")
}

func TestCreateOrchestrationDefinition_EmptyActivities(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	// Create an orchestration definition with no activities
	orchestrationDef := &api.OrchestrationDefinition{
		Type:       model.OrchestrationType("empty-orchestration"),
		Active:     true,
		Schema:     map[string]any{"type": "object"},
		Activities: []api.Activity{}, // Empty activities slice
	}

	result, err := manager.CreateOrchestrationDefinition(ctx, orchestrationDef)

	require.NoError(t, err, "CreateOrchestrationDefinition should succeed with empty activities")
	assert.NotNil(t, result, "Result should not be nil")
	assert.Equal(t, 0, len(result.Activities), "Should have 0 activities")
}

func TestCreateOrchestrationDefinition_StoreError(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	// Create and store the orchestration definition first
	orchestrationDef := &api.OrchestrationDefinition{
		Type:       model.OrchestrationType("duplicate-orchestration"),
		Active:     true,
		Schema:     map[string]any{"type": "object"},
		Activities: []api.Activity{},
	}

	_, err := store.StoreOrchestrationDefinition(context.Background(), orchestrationDef)
	require.NoError(t, err, "First store should succeed")

	// Attempt to store the same orchestration definition again
	result, err := manager.CreateOrchestrationDefinition(ctx, orchestrationDef)

	require.Error(t, err, "CreateOrchestrationDefinition should fail on duplicate")
	assert.Nil(t, result, "Result should be nil on error")

	// Verify the error is a conflict error
	require.True(t, errors.Is(err, types.ErrConflict), "Error should be a ConflictError")
}

func TestCreateActivityDefinition_Success(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	activityDef := &api.ActivityDefinition{
		Type:         "test-activity",
		Description:  "Test activity definition",
		InputSchema:  map[string]any{"type": "object", "properties": map[string]any{"input": map[string]any{"type": "string"}}},
		OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"output": map[string]any{"type": "string"}}},
	}

	result, err := manager.CreateActivityDefinition(ctx, activityDef)

	require.NoError(t, err, "CreateActivityDefinition should succeed")
	assert.NotNil(t, result, "Result should not be nil")
	assert.Equal(t, activityDef.Type, result.Type, "Activity type should match")
	assert.Equal(t, activityDef.Description, result.Description, "Description should match")
	assert.Equal(t, activityDef.InputSchema, result.InputSchema, "InputSchema should match")
	assert.Equal(t, activityDef.OutputSchema, result.OutputSchema, "OutputSchema should match")
}

func TestCreateActivityDefinition_Duplicate(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	activityDef := &api.ActivityDefinition{
		Type:         "duplicate-activity",
		Description:  "Test activity definition",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}

	_, err := store.StoreActivityDefinition(context.Background(), activityDef)
	require.NoError(t, err, "First store should succeed")

	// Try to create the same activity definition again
	result, err := manager.CreateActivityDefinition(ctx, activityDef)

	require.Error(t, err, "CreateActivityDefinition should fail on duplicate")
	assert.Nil(t, result, "Result should be nil on error")

	require.True(t, errors.Is(err, types.ErrConflict), "Error should be a ConflictError")
}

func TestIntegration_CompleteWorkflow(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	// Create activity definitions first
	activityDef1 := &api.ActivityDefinition{
		Type:         "prepare",
		Description:  "Prepare resources",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}

	activityDef2 := &api.ActivityDefinition{
		Type:         "deploy",
		Description:  "Send application",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}

	result1, err := manager.CreateActivityDefinition(ctx, activityDef1)
	require.NoError(t, err, "Should create first activity definition")
	assert.Equal(t, activityDef1.Type, result1.Type, "First activity type should match")

	result2, err := manager.CreateActivityDefinition(ctx, activityDef2)
	require.NoError(t, err, "Should create second activity definition")
	assert.Equal(t, activityDef2.Type, result2.Type, "Second activity type should match")

	// Create an orchestration definition that uses both activities
	orchestrationDef := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("full-orchestration"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "prepare-step",
				Type:      "prepare",
				Inputs:    []api.MappingEntry{{Source: "config", Target: "configuration"}},
				DependsOn: []string{},
			},
			{
				ID:        "orchestration-step",
				Type:      "deploy",
				Inputs:    []api.MappingEntry{{Source: "artifact", Target: "deployment_artifact"}},
				DependsOn: []string{"prepare-step"},
			},
		},
	}

	result3, err := manager.CreateOrchestrationDefinition(ctx, orchestrationDef)

	require.NoError(t, err, "Should create orchestration definition")
	assert.NotNil(t, result3, "Result should not be nil")
	assert.Equal(t, orchestrationDef.Type, result3.Type, "Orchestration type should match")
	assert.Equal(t, 2, len(result3.Activities), "Should have 2 activities")

	// Verify the activities are correctly referenced
	activityTypes := make(map[api.ActivityType]bool)
	for _, activity := range result3.Activities {
		activityTypes[activity.Type] = true
	}
	assert.True(t, activityTypes["prepare"], "Should contain prepare activity")
	assert.True(t, activityTypes["deploy"], "Should contain deploy activity")

	// Verify stored definitions can be retrieved
	retrievedOrchestration, err := store.FindOrchestrationDefinition(context.Background(), orchestrationDef.Type)
	require.NoError(t, err, "Should retrieve orchestration definition")
	assert.Equal(t, orchestrationDef.Type, retrievedOrchestration.Type, "Retrieved deployment type should match")

	retrievedActivity1, err := store.FindActivityDefinition(ctx, "prepare")
	require.NoError(t, err, "Should retrieve first activity definition")
	assert.Equal(t, api.ActivityType("prepare"), retrievedActivity1.Type, "Retrieved activity type should match")

	retrievedActivity2, err := store.FindActivityDefinition(ctx, "deploy")
	require.NoError(t, err, "Should retrieve second activity definition")
	assert.Equal(t, api.ActivityType("deploy"), retrievedActivity2.Type, "Retrieved activity type should match")
}

func TestDeleteOrchestrationDefinition_Success(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext:         cstore.NoOpTransactionContext{},
		store:              store,
		orchestrationStore: memorystore2.NewInMemoryEntityStore[*api.OrchestrationEntry](),
	}

	orchestrationDef := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("test-orchestration-to-delete"),
		Description: "Orchestration to be deleted",
		Active:      true,
		Schema:      map[string]any{"type": "object"},
		Activities:  []api.Activity{},
		TemplateRef: "test-template-ref",
	}

	ctx := context.Background()

	_, err := store.StoreOrchestrationDefinition(ctx, orchestrationDef)
	require.NoError(t, err, "Failed to store orchestration definition")

	err = manager.DeleteOrchestrationDefinition(ctx, orchestrationDef.TemplateRef)

	require.NoError(t, err, "DeleteOrchestrationDefinition should succeed")

	exists, err := store.ExistsOrchestrationDefinition(ctx, orchestrationDef.Type)
	require.NoError(t, err, "Failed to check existence")
	assert.False(t, exists, "Orchestration definition should no longer exist")
}

func TestDeleteOrchestrationDefinition_NotFound(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}

	ctx := context.Background()
	err := manager.DeleteOrchestrationDefinition(ctx, "non-existent-ref")

	require.Error(t, err, "DeleteOrchestrationDefinition should fail for non-existent orchestration")
	assert.True(t, errors.Is(err, types.ErrNotFound), "Error should be ErrNotFound")
}

func TestDeleteOrchestrationDefinition_ExistsCheckError(t *testing.T) {
	mockStore := newMockDefinitionStore()
	mockStore.simulateError("findByPredicate")
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      mockStore,
	}

	ctx := context.Background()
	err := manager.DeleteOrchestrationDefinition(ctx, "test-template-id")

	require.Error(t, err, "DeleteOrchestrationDefinition should fail when exists check errors")
	assert.True(t, types.IsRecoverable(err), "Error should be a RecoverableError")
	assert.Contains(t, err.Error(), "failed to check orchestration definition for template-ref",
		"Error message should indicate existence check failure")
}

func TestDeleteOrchestrationDefinition_DeleteError(t *testing.T) {
	mockStore := newMockDefinitionStore()
	mockStore.setOrchestrationDefinition(api.OrchestrationDefinition{})
	mockStore.simulateError("deleteOrchestration")
	manager := definitionManager{
		trxContext:         cstore.NoOpTransactionContext{},
		store:              mockStore,
		orchestrationStore: memorystore2.NewInMemoryEntityStore[*api.OrchestrationEntry](),
	}

	ctx := context.Background()
	err := manager.DeleteOrchestrationDefinition(ctx, "test-template-id")

	require.Error(t, err, "DeleteOrchestrationDefinition should fail when delete errors")
	assert.True(t, types.IsRecoverable(err), "Error should be a RecoverableError")
	assert.Contains(t, err.Error(), "failed to delete orchestration definition for template-ref",
		"Error message should indicate deletion failure")
}

func TestDeleteOrchestrationDefinition_DeleteReturnsFalse(t *testing.T) {
	mockStore := newMockDefinitionStore()
	mockStore.setOrchestrationDefinition(api.OrchestrationDefinition{})
	mockStore.setDeleteOrchestrationDefinitionReturned(false)
	manager := definitionManager{
		trxContext:         cstore.NoOpTransactionContext{},
		store:              mockStore,
		orchestrationStore: memorystore2.NewInMemoryEntityStore[*api.OrchestrationEntry](),
	}

	ctx := context.Background()
	err := manager.DeleteOrchestrationDefinition(ctx, "test-template-id")

	require.Error(t, err, "DeleteOrchestrationDefinition should fail when delete returns false")
	assert.True(t, types.IsClientError(err), "Error should be a ClientError")
	assert.Contains(t, err.Error(), "unable to delete orchestration definition template-ref",
		"Error message should indicate deletion failed")
}

func TestDeleteOrchestrationDefinition_Multiple(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext:         cstore.NoOpTransactionContext{},
		store:              store,
		orchestrationStore: memorystore2.NewInMemoryEntityStore[*api.OrchestrationEntry](),
	}

	ctx := context.Background()

	// Create multiple orchestration definitions
	orchestration1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-1"),
		Active:      true,
		Schema:      map[string]any{"type": "object"},
		Activities:  []api.Activity{},
		TemplateRef: "template-1",
	}
	orchestration2 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-2"),
		Active:      true,
		Schema:      map[string]any{"type": "object"},
		Activities:  []api.Activity{},
		TemplateRef: "template-2",
	}
	orchestration3 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-3"),
		Active:      true,
		Schema:      map[string]any{"type": "object"},
		Activities:  []api.Activity{},
		TemplateRef: "template-3",
	}

	_, err := store.StoreOrchestrationDefinition(ctx, orchestration1)
	require.NoError(t, err)
	_, err = store.StoreOrchestrationDefinition(ctx, orchestration2)
	require.NoError(t, err)
	_, err = store.StoreOrchestrationDefinition(ctx, orchestration3)
	require.NoError(t, err)

	// Delete first one
	err = manager.DeleteOrchestrationDefinition(ctx, orchestration1.TemplateRef)
	require.NoError(t, err, "First deletion should succeed")

	exists, err := store.ExistsOrchestrationDefinition(ctx, orchestration1.Type)
	require.NoError(t, err)
	assert.False(t, exists, "First orchestration should be deleted")

	// Verify others still exist
	exists, err = store.ExistsOrchestrationDefinition(ctx, orchestration2.Type)
	require.NoError(t, err)
	assert.True(t, exists, "Second orchestration should still exist")

	exists, err = store.ExistsOrchestrationDefinition(ctx, orchestration3.Type)
	require.NoError(t, err)
	assert.True(t, exists, "Third orchestration should still exist")

	// Delete second one
	err = manager.DeleteOrchestrationDefinition(ctx, orchestration2.TemplateRef)
	require.NoError(t, err, "Second deletion should succeed")

	exists, err = store.ExistsOrchestrationDefinition(ctx, orchestration2.Type)
	require.NoError(t, err)
	assert.False(t, exists, "Second orchestration should be deleted")

	// Verify third still exists
	exists, err = store.ExistsOrchestrationDefinition(ctx, orchestration3.Type)
	require.NoError(t, err)
	assert.True(t, exists, "Third orchestration should still exist")
}

func TestDeleteOrchestrationDefinition_WithOrchestrations(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	orchestrationIndex := memorystore2.NewInMemoryEntityStore[*api.OrchestrationEntry]()
	manager := definitionManager{
		trxContext:         cstore.NoOpTransactionContext{},
		store:              store,
		orchestrationStore: orchestrationIndex,
	}

	orchestrationDef := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("test-orchestration-to-delete"),
		Description: "Orchestration to be deleted",
		Active:      true,
		Schema:      map[string]any{"type": "object"},
		Activities:  []api.Activity{},
		TemplateRef: "test-template-ref",
	}

	ctx := t.Context()

	orchestrationDef, err := store.StoreOrchestrationDefinition(ctx, orchestrationDef)
	require.NoError(t, err, "Failed to store orchestration definition")

	_, err = orchestrationIndex.Create(ctx, &api.OrchestrationEntry{
		ID:                uuid.NewString(),
		Version:           0,
		CorrelationID:     "test-correlationId",
		State:             0,
		StateTimestamp:    time.Time{},
		CreatedTimestamp:  time.Time{},
		OrchestrationType: model.VPADeployType,
		DefinitionID:      orchestrationDef.GetID(),
	})
	require.NoError(t, err, "Failed to create orchestration entry")

	err = manager.DeleteOrchestrationDefinition(ctx, orchestrationDef.TemplateRef)
	require.Error(t, err, "Failed to delete orchestration definition")

}

func TestQueryOrchestrationDefinitions(t *testing.T) {

	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := t.Context()
	orchestration1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-1"),
		Active:      true,
		Schema:      map[string]any{"type": "object"},
		Activities:  []api.Activity{},
		TemplateRef: "template-1",
	}
	orchestration2 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-2"),
		Active:      true,
		Schema:      map[string]any{"type": "object"},
		Activities:  []api.Activity{},
		TemplateRef: "template-1",
	}
	_, err := store.StoreOrchestrationDefinition(ctx, orchestration1)
	require.NoError(t, err)
	_, err = store.StoreOrchestrationDefinition(ctx, orchestration2)
	require.NoError(t, err)

	predicate := query.Contains("TemplateRef", "template")
	definitions, err := manager.QueryOrchestrationDefinitions(ctx, predicate)
	require.NoError(t, err)

	assert.Len(t, definitions, 2)
	assert.ElementsMatch(t, definitions, []api.OrchestrationDefinition{*orchestration1, *orchestration2})
}

func TestDeleteActivityDefinition_Success(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}

	activityDef := &api.ActivityDefinition{
		Type:         "test-activity-to-delete",
		Description:  "Activity to be deleted",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}

	ctx := context.Background()

	_, err := store.StoreActivityDefinition(ctx, activityDef)
	require.NoError(t, err, "Failed to store activity definition")

	err = manager.DeleteActivityDefinition(ctx, activityDef.Type)

	require.NoError(t, err, "DeleteActivityDefinition should succeed")

	exists, err := store.ExistsActivityDefinition(ctx, activityDef.Type)
	require.NoError(t, err, "Failed to check existence")
	assert.False(t, exists, "Activity definition should no longer exist")
}

func TestDeleteActivityDefinition_NotFound(t *testing.T) {
	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}

	ctx := context.Background()
	err := manager.DeleteActivityDefinition(ctx, "non-existent-activity")

	require.Error(t, err, "DeleteActivityDefinition should fail for non-existent activity")
	assert.True(t, errors.Is(err, types.ErrNotFound), "Error should be ErrNotFound")
}

func TestDeleteActivityDefinition_ReferencedByOrchestration(t *testing.T) {

	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}

	// Create an activity definition
	activityDef := &api.ActivityDefinition{
		Type:         "referenced-activity",
		Description:  "Activity referenced by orchestration",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}

	ctx := context.Background()
	_, err := store.StoreActivityDefinition(ctx, activityDef)

	require.NoError(t, err, "Failed to store activity definition")

	// Create an orchestration definition that references the activity
	orchestrationDef := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("test-orchestration-with-ref"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "activity-1",
				Type:      activityDef.Type,
				Inputs:    []api.MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
		},
	}
	_, err = store.StoreOrchestrationDefinition(ctx, orchestrationDef)
	require.NoError(t, err, "Failed to store orchestration definition")

	err = manager.DeleteActivityDefinition(ctx, activityDef.Type)

	// Then: Should fail with a client error indicating it's referenced
	require.Error(t, err, "DeleteActivityDefinition should fail when activity is referenced")
	assert.True(t, types.IsClientError(err), "Error should be a ClientError")
	assert.Contains(t, err.Error(), "referenced by an orchestration definition",
		"Error message should indicate the activity is referenced")
	assert.Contains(t, err.Error(), activityDef.Type,
		"Error message should contain the activity type")

	// Verify the activity definition still exists
	exists, err := store.ExistsActivityDefinition(ctx, activityDef.Type)
	require.NoError(t, err, "Failed to check existence")
	assert.True(t, exists, "Activity definition should still exist after failed deletion")
}

func TestDeleteActivityDefinition_ExistsCheckError(t *testing.T) {
	mockStore := newMockDefinitionStore()
	mockStore.simulateError("existsActivity")
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      mockStore,
	}

	ctx := context.Background()
	err := manager.DeleteActivityDefinition(ctx, "test-activity")

	require.Error(t, err, "DeleteActivityDefinition should fail when exists check errors")
	assert.True(t, types.IsRecoverable(err), "Error should be a RecoverableError")
	assert.Contains(t, err.Error(), "failed to check activity definition for type",
		"Error message should indicate existence check failure")
}

func TestDeleteActivityDefinition_ReferenceCheckError(t *testing.T) {
	mockStore := newMockDefinitionStore()
	mockStore.setActivityExists(true)
	mockStore.simulateError("referenced")
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      mockStore,
	}

	ctx := context.Background()
	err := manager.DeleteActivityDefinition(ctx, "test-activity")

	// Then: Should return a recoverable error
	require.Error(t, err, "DeleteActivityDefinition should fail when reference check errors")
	assert.True(t, types.IsRecoverable(err), "Error should be a RecoverableError")
	assert.Contains(t, err.Error(), "failed to check activity definition references for type",
		"Error message should indicate reference check failure")
}

func TestDeleteActivityDefinition_DeleteError(t *testing.T) {

	mockStore := newMockDefinitionStore()
	mockStore.setActivityExists(true)
	mockStore.simulateError("deleteActivity")
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      mockStore,
	}

	ctx := context.Background()
	err := manager.DeleteActivityDefinition(ctx, "test-activity")

	require.Error(t, err, "DeleteActivityDefinition should fail when delete errors")
	assert.True(t, types.IsRecoverable(err), "Error should be a RecoverableError")
	assert.Contains(t, err.Error(), "failed to check activity definition references for type",
		"Error message should mention the operation that failed")
}

func TestDeleteActivityDefinition_DeleteReturnsFalse(t *testing.T) {
	mockStore := newMockDefinitionStore()
	mockStore.setActivityExists(true)
	mockStore.setDeleteActivityDefinitionReturned(false)
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      mockStore,
	}

	ctx := context.Background()
	err := manager.DeleteActivityDefinition(ctx, "test-activity")

	require.Error(t, err, "DeleteActivityDefinition should fail when delete returns false")
	assert.True(t, types.IsClientError(err), "Error should be a ClientError")
	assert.Contains(t, err.Error(), "unable to delete activity definition type",
		"Error message should indicate deletion failed")
}

func TestDeleteActivityDefinition_MultipleReferences(t *testing.T) {

	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}

	ctx := context.Background()

	// Create an activity definition
	activityDef := &api.ActivityDefinition{
		Type:         "shared-activity",
		Description:  "Activity used by multiple orchestrations",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}
	_, err := store.StoreActivityDefinition(ctx, activityDef)
	require.NoError(t, err, "Failed to store activity definition")

	// Create first orchestration that references the activity
	orchestrationDef1 := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("orchestration-1"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "activity-1",
				Type:      activityDef.Type,
				Inputs:    []api.MappingEntry{{Source: "input", Target: "output"}},
				DependsOn: []string{},
			},
		},
	}
	_, err = store.StoreOrchestrationDefinition(ctx, orchestrationDef1)
	require.NoError(t, err, "Failed to store first orchestration")

	// Create second orchestration that also references the activity
	orchestrationDef2 := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("orchestration-2"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "activity-2",
				Type:      activityDef.Type,
				Inputs:    []api.MappingEntry{{Source: "input", Target: "output"}},
				DependsOn: []string{},
			},
		},
	}
	_, err = store.StoreOrchestrationDefinition(ctx, orchestrationDef2)
	require.NoError(t, err, "Failed to store second orchestration")

	err = manager.DeleteActivityDefinition(ctx, activityDef.Type)

	require.Error(t, err, "DeleteActivityDefinition should fail when activity is referenced")
	assert.True(t, types.IsClientError(err), "Error should be a ClientError")
	assert.Contains(t, err.Error(), "referenced by an orchestration definition",
		"Error message should indicate it's referenced")
}

func TestDeleteActivityDefinition_AfterOrchestrationDeletion(t *testing.T) {

	store := memorystore.NewDefinitionStore()
	manager := definitionManager{
		trxContext: cstore.NoOpTransactionContext{},
		store:      store,
	}
	ctx := context.Background()

	// Create an activity definition
	activityDef := &api.ActivityDefinition{
		Type:         "deletable-activity",
		Description:  "Activity that can be deleted",
		InputSchema:  map[string]any{"type": "object"},
		OutputSchema: map[string]any{"type": "object"},
	}
	_, err := store.StoreActivityDefinition(ctx, activityDef)
	require.NoError(t, err, "Failed to store activity definition")

	// Create an orchestration that references the activity
	orchestrationDef := &api.OrchestrationDefinition{
		Type:   model.OrchestrationType("temp-orchestration"),
		Active: true,
		Schema: map[string]any{"type": "object"},
		Activities: []api.Activity{
			{
				ID:        "activity-1",
				Type:      activityDef.Type,
				Inputs:    []api.MappingEntry{{Source: "input", Target: "output"}},
				DependsOn: []string{},
			},
		},
	}
	_, err = store.StoreOrchestrationDefinition(ctx, orchestrationDef)
	require.NoError(t, err, "Failed to store orchestration")

	// First deletion attempt should fail
	err = manager.DeleteActivityDefinition(ctx, activityDef.Type)
	require.Error(t, err, "First deletion should fail because activity is referenced")

	// Delete the orchestration definition
	deleted, err := store.DeleteOrchestrationDefinition(ctx, orchestrationDef.Type)
	require.NoError(t, err, "Failed to delete orchestration")
	assert.True(t, deleted, "Orchestration should be deleted")

	err = manager.DeleteActivityDefinition(ctx, activityDef.Type)

	require.NoError(t, err, "DeleteActivityDefinition should succeed after orchestration is deleted")

	exists, err := store.ExistsActivityDefinition(ctx, activityDef.Type)
	require.NoError(t, err, "Failed to check existence")
	assert.False(t, exists, "Activity definition should no longer exist")
}

// Helper mock store for testing error scenarios
type mockDefinitionStore struct {
	simulatedErrors map[string]error
	state           map[string]any
}

func newMockDefinitionStore() *mockDefinitionStore {
	return &mockDefinitionStore{
		simulatedErrors: make(map[string]error),
		state:           make(map[string]any),
	}
}

// simulateError sets an error condition for a given operation
func (m *mockDefinitionStore) simulateError(operation string) {
	m.simulatedErrors[operation] = errors.New("simulated error for " + operation)
}

// setDeleteOrchestrationDefinitionReturned sets whether delete returns true
func (m *mockDefinitionStore) setDeleteOrchestrationDefinitionReturned(returned bool) {
	m.state["deleteOrchestrationReturned"] = returned
}

// setOrchestrationExists sets whether the orchestration exists
func (m *mockDefinitionStore) setOrchestrationExists(exists bool) {
	m.state["orchestrationExists"] = exists
}

func (m *mockDefinitionStore) setOrchestrationDefinition(definition api.OrchestrationDefinition) {
	m.state["orchestrationDefinition"] = definition
}

func (m *mockDefinitionStore) ExistsOrchestrationDefinition(context.Context, model.OrchestrationType) (bool, error) {
	if err, exists := m.simulatedErrors["existsOrchestration"]; exists {
		return false, err
	}
	return m.state["orchestrationExists"].(bool), nil
}

func (m *mockDefinitionStore) DeleteOrchestrationDefinition(context.Context, model.OrchestrationType) (bool, error) {
	if err, exists := m.simulatedErrors["deleteOrchestration"]; exists {
		return false, err
	}
	return m.state["deleteOrchestrationReturned"].(bool), nil
}

// setActivityExists sets whether the activity exists
func (m *mockDefinitionStore) setActivityExists(exists bool) {
	m.state["activityExists"] = exists
}

// setDeleteActivityDefinitionReturned sets whether delete returns true
func (m *mockDefinitionStore) setDeleteActivityDefinitionReturned(returned bool) {
	m.state["deleteActivityReturned"] = returned
}

func (m *mockDefinitionStore) FindOrchestrationDefinition(context.Context, model.OrchestrationType) (*api.OrchestrationDefinition, error) {
	return nil, types.ErrNotFound
}

func (m *mockDefinitionStore) FindOrchestrationDefinitionsByPredicate(context.Context, query.Predicate) iter.Seq2[api.OrchestrationDefinition, error] {
	if err, exists := m.simulatedErrors["findByPredicate"]; exists {
		// return an iterator that yields an error
		return func(yield func(api.OrchestrationDefinition, error) bool) {
			yield(api.OrchestrationDefinition{}, err)
		}
	}
	// return an iterator that yields a result
	return func(yield func(api.OrchestrationDefinition, error) bool) {
		yield(m.state["orchestrationDefinition"].(api.OrchestrationDefinition), nil)
	}
}

func (m *mockDefinitionStore) FindActivityDefinition(context.Context, api.ActivityType) (*api.ActivityDefinition, error) {
	return nil, types.ErrNotFound
}

func (m *mockDefinitionStore) FindActivityDefinitionsByPredicate(context.Context, query.Predicate) iter.Seq2[api.ActivityDefinition, error] {
	return nil
}

func (m *mockDefinitionStore) ExistsActivityDefinition(context.Context, api.ActivityType) (bool, error) {
	if err, exists := m.simulatedErrors["existsActivity"]; exists {
		return false, err
	}
	return m.state["activityExists"].(bool), nil
}

func (m *mockDefinitionStore) StoreOrchestrationDefinition(
	_ context.Context,
	definition *api.OrchestrationDefinition) (*api.OrchestrationDefinition, error) {
	return definition, nil
}

func (m *mockDefinitionStore) StoreActivityDefinition(
	_ context.Context,
	definition *api.ActivityDefinition) (*api.ActivityDefinition, error) {
	return definition, nil
}

func (m *mockDefinitionStore) ActivityDefinitionReferences(ctx context.Context, activityType api.ActivityType) ([]string, error) {
	if err, exists := m.simulatedErrors["referenced"]; exists {
		return nil, err
	}
	return []string{}, nil
}

func (m *mockDefinitionStore) DeleteActivityDefinition(context.Context, api.ActivityType) (bool, error) {
	if err, exists := m.simulatedErrors["deleteActivity"]; exists {
		return false, err
	}
	return m.state["deleteActivityReturned"].(bool), nil
}

func (m *mockDefinitionStore) ListOrchestrationDefinitions(context.Context) ([]api.OrchestrationDefinition, error) {
	return nil, nil
}

func (m *mockDefinitionStore) ListActivityDefinitions(context.Context) ([]api.ActivityDefinition, error) {
	return nil, nil
}
