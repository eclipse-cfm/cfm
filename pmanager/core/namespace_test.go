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
	"testing"

	memorystore2 "github.com/eclipse-cfm/cfm/common/memorystore"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/memorystore"
	"github.com/eclipse-cfm/cfm/pmanager/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newNamespaceProvisionManager(t *testing.T, orchestrator api.Orchestrator, definitions ...*api.OrchestrationDefinition) *provisionManager {
	t.Helper()
	definitionStore := memorystore.NewDefinitionStore()
	for _, definition := range definitions {
		_, err := definitionStore.StoreOrchestrationDefinition(context.Background(), definition)
		require.NoError(t, err)
	}
	return &provisionManager{
		orchestrator: orchestrator,
		store:        definitionStore,
		index:        memorystore2.NewInMemoryEntityStore[*api.OrchestrationEntry](),
		monitor:      &system.NoopMonitor{},
		trxContext:   store.NoOpTransactionContext{},
	}
}

func namespacedDefinition(namespace string, activityType api.ActivityType) *api.OrchestrationDefinition {
	return &api.OrchestrationDefinition{
		Namespace:  namespace,
		Type:       "test-type",
		Activities: []api.Activity{{ID: "activity1", Type: activityType}},
	}
}

func TestProvisionManager_Start_UsesDefinitionOfNamespace(t *testing.T) {
	mockOrch := mocks.NewMockOrchestrator(t)
	mockOrch.EXPECT().GetOrchestration(mock.Anything, "second", "deployment").Return(nil, nil)
	mockOrch.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(orch *api.Orchestration) bool {
		return orch.Namespace == "second" && orch.DefinitionID == "test-type" && orch.Steps[0].Activities[0].Type == "second-activity"
	})).Return(nil)
	pm := newNamespaceProvisionManager(t, mockOrch, namespacedDefinition("", "default-activity"), namespacedDefinition("second", "second-activity"))

	result, err := pm.Start(context.Background(), &model.OrchestrationManifest{ID: "deployment", OrchestrationType: "test-type", Namespace: "second"})

	require.NoError(t, err)
	assert.Equal(t, "second", result.Namespace)
}

func TestProvisionManager_Start_DefaultNamespace(t *testing.T) {
	mockOrch := mocks.NewMockOrchestrator(t)
	mockOrch.EXPECT().GetOrchestration(mock.Anything, model.DefaultNamespace, "deployment").Return(nil, nil)
	mockOrch.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(orch *api.Orchestration) bool {
		return orch.Namespace == model.DefaultNamespace && orch.Steps[0].Activities[0].Type == "default-activity"
	})).Return(nil)
	pm := newNamespaceProvisionManager(t, mockOrch, namespacedDefinition("", "default-activity"), namespacedDefinition("second", "second-activity"))

	manifest := &model.OrchestrationManifest{ID: "deployment", OrchestrationType: "test-type"}
	result, err := pm.Start(context.Background(), manifest)

	require.NoError(t, err)
	assert.Equal(t, model.DefaultNamespace, result.Namespace)
	assert.Equal(t, model.DefaultNamespace, manifest.Namespace)
}

func TestProvisionManager_Start_TypeOfOtherNamespaceNotFound(t *testing.T) {
	pm := newNamespaceProvisionManager(t, mocks.NewMockOrchestrator(t), namespacedDefinition("second", "second-activity"))

	_, err := pm.Start(context.Background(), &model.OrchestrationManifest{ID: "deployment", OrchestrationType: "test-type"})

	require.Error(t, err)
	assert.True(t, types.IsClientError(err))
	assert.Contains(t, err.Error(), "not found in namespace 'default'")
}

func TestProvisionManager_Start_RejectsInvalidNamespace(t *testing.T) {
	pm := newNamespaceProvisionManager(t, mocks.NewMockOrchestrator(t))

	_, err := pm.Start(context.Background(), &model.OrchestrationManifest{ID: "deployment", OrchestrationType: "test-type", Namespace: "second.other"})

	require.Error(t, err)
	assert.True(t, types.IsClientError(err))
}

func TestProvisionManager_Start_DeduplicatesWithinNamespace(t *testing.T) {
	// an orchestration of the default namespace with the same ID does not keep the second namespace from starting one,
	// because de-duplication only looks up the orchestration of the manifest's namespace
	mockOrch := mocks.NewMockOrchestrator(t)
	mockOrch.EXPECT().GetOrchestration(mock.Anything, "second", "deployment").Return(nil, nil)
	mockOrch.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(orch *api.Orchestration) bool {
		return orch.ID == "deployment" && orch.Namespace == "second"
	})).Return(nil)
	pm := newNamespaceProvisionManager(t, mockOrch, namespacedDefinition("second", "second-activity"))

	_, err := pm.Start(context.Background(), &model.OrchestrationManifest{ID: "deployment", OrchestrationType: "test-type", Namespace: "second"})

	require.NoError(t, err)
}

func TestProvisionManager_GetOrchestration_OnlyInNamespace(t *testing.T) {
	stored := &api.Orchestration{ID: "o1", Namespace: "second"}
	mockOrch := mocks.NewMockOrchestrator(t)
	mockOrch.EXPECT().GetOrchestration(mock.Anything, "second", "o1").Return(stored, nil)
	mockOrch.EXPECT().GetOrchestration(mock.Anything, model.DefaultNamespace, "o1").Return(nil, nil)
	pm := newNamespaceProvisionManager(t, mockOrch)
	ctx := context.Background()

	found, err := pm.GetOrchestration(ctx, "second", "o1")
	require.NoError(t, err)
	assert.Same(t, stored, found)

	found, err = pm.GetOrchestration(ctx, model.DefaultNamespace, "o1")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestProvisionManager_GetOrchestration_IgnoresStateOfOtherNamespace(t *testing.T) {
	// state under the key of a namespace that names another namespace was not written by the engine
	mockOrch := mocks.NewMockOrchestrator(t)
	mockOrch.EXPECT().GetOrchestration(mock.Anything, "second", "o1").Return(&api.Orchestration{ID: "o1", Namespace: "third"}, nil)
	pm := newNamespaceProvisionManager(t, mockOrch)

	found, err := pm.GetOrchestration(context.Background(), "second", "o1")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestProvisionManager_QueryAndCountOrchestrations_OnlyInNamespace(t *testing.T) {
	pm := newNamespaceProvisionManager(t, mocks.NewMockOrchestrator(t))
	ctx := context.Background()
	for _, entry := range []*api.OrchestrationEntry{
		{ID: "o1", Namespace: model.DefaultNamespace, OrchestrationType: "test-type"},
		{ID: "o2", Namespace: "second", OrchestrationType: "test-type"},
		{ID: "o3", Namespace: "second", OrchestrationType: "other-type"},
	} {
		_, err := pm.index.Create(ctx, entry)
		require.NoError(t, err)
	}

	count, err := pm.CountOrchestrations(ctx, "second", query.Eq("orchestrationType", "test-type"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	var ids []string
	for entry, err := range pm.QueryOrchestrations(ctx, "second", &query.MatchAllPredicate{}, store.PaginationOptions{}) {
		require.NoError(t, err)
		ids = append(ids, entry.ID)
	}
	assert.ElementsMatch(t, []string{"o2", "o3"}, ids)

	count, err = pm.CountOrchestrations(ctx, "", &query.MatchAllPredicate{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func newNamespaceDefinitionManager() definitionManager {
	return definitionManager{
		trxContext:         store.NoOpTransactionContext{},
		store:              memorystore.NewDefinitionStore(),
		orchestrationStore: memorystore2.NewInMemoryEntityStore[*api.OrchestrationEntry](),
	}
}

func TestDefinitionManager_OrchestrationDefinitionUsesActivitiesOfItsNamespace(t *testing.T) {
	manager := newNamespaceDefinitionManager()
	ctx := context.Background()
	_, err := manager.CreateActivityDefinition(ctx, &api.ActivityDefinition{Type: "test-activity"})
	require.NoError(t, err)

	_, err = manager.CreateOrchestrationDefinition(ctx, namespacedDefinition("second", "test-activity"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "activity type 'test-activity' not found in namespace 'second'")

	_, err = manager.CreateActivityDefinition(ctx, &api.ActivityDefinition{Namespace: "second", Type: "test-activity"})
	require.NoError(t, err)
	created, err := manager.CreateOrchestrationDefinition(ctx, namespacedDefinition("second", "test-activity"))
	require.NoError(t, err)
	assert.Equal(t, "second", created.Namespace)

	definitions, err := manager.GetOrchestrationDefinitions(ctx, model.DefaultNamespace)
	require.NoError(t, err)
	assert.Empty(t, definitions)
	definitions, err = manager.GetOrchestrationDefinitions(ctx, "second")
	require.NoError(t, err)
	assert.Len(t, definitions, 1)
}

func TestDefinitionManager_CreateRejectsInvalidNamespace(t *testing.T) {
	manager := newNamespaceDefinitionManager()
	ctx := context.Background()

	_, err := manager.CreateActivityDefinition(ctx, &api.ActivityDefinition{Namespace: "a.b", Type: "test-activity"})
	require.Error(t, err)
	assert.True(t, types.IsClientError(err))

	_, err = manager.CreateOrchestrationDefinition(ctx, namespacedDefinition("a.b", "test-activity"))
	require.Error(t, err)
	assert.True(t, types.IsClientError(err))
}

func TestDefinitionManager_DeleteOnlyInNamespace(t *testing.T) {
	manager := newNamespaceDefinitionManager()
	ctx := context.Background()
	for _, namespace := range []string{model.DefaultNamespace, "second"} {
		_, err := manager.CreateActivityDefinition(ctx, &api.ActivityDefinition{Namespace: namespace, Type: "test-activity"})
		require.NoError(t, err)
		definition := namespacedDefinition(namespace, "test-activity")
		definition.TemplateRef = "template-1"
		_, err = manager.CreateOrchestrationDefinition(ctx, definition)
		require.NoError(t, err)
	}
	// an orchestration of the default namespace does not block deleting the definition of the second namespace
	_, err := manager.orchestrationStore.Create(ctx, &api.OrchestrationEntry{ID: "o1", Namespace: model.DefaultNamespace, DefinitionID: "test-type"})
	require.NoError(t, err)

	err = manager.DeleteOrchestrationDefinition(ctx, model.DefaultNamespace, "template-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ongoing orchestrations")

	require.NoError(t, manager.DeleteOrchestrationDefinition(ctx, "second", "template-1"))
	err = manager.DeleteOrchestrationDefinition(ctx, "second", "template-1")
	require.ErrorIs(t, err, types.ErrNotFound)

	// the activity of the default namespace is still referenced, the one of the second namespace is not
	err = manager.DeleteActivityDefinition(ctx, model.DefaultNamespace, "test-activity")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is referenced by an orchestration definition")
	require.NoError(t, manager.DeleteActivityDefinition(ctx, "second", "test-activity"))

	byTemplate, err := manager.GetOrchestrationDefinitionsByTemplate(ctx, model.DefaultNamespace, "template-1")
	require.NoError(t, err)
	assert.Len(t, byTemplate, 1)
	activities, err := manager.GetActivityDefinitions(ctx, "second")
	require.NoError(t, err)
	assert.Empty(t, activities)
}
