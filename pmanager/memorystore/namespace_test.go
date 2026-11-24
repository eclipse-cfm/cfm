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

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefinitionStore_SameTypeInTwoNamespaces(t *testing.T) {
	ctx := context.Background()
	store := NewDefinitionStore()

	for _, namespace := range []string{"", "second"} {
		_, err := store.StoreActivityDefinition(ctx, &api.ActivityDefinition{Namespace: namespace, Type: "test-activity", Description: "activity " + namespace})
		require.NoError(t, err)
		_, err = store.StoreOrchestrationDefinition(ctx, &api.OrchestrationDefinition{
			Namespace:   namespace,
			Type:        "test.deploy",
			TemplateRef: "template-1",
			Description: "orchestration " + namespace,
			Activities:  []api.Activity{{ID: "a1", Type: "test-activity"}},
		})
		require.NoError(t, err)
	}

	defaultDefinition, err := store.FindOrchestrationDefinition(ctx, model.DefaultNamespace, "test.deploy")
	require.NoError(t, err)
	assert.Equal(t, model.DefaultNamespace, defaultDefinition.Namespace)
	assert.Equal(t, "orchestration ", defaultDefinition.Description)

	secondDefinition, err := store.FindOrchestrationDefinition(ctx, "second", "test.deploy")
	require.NoError(t, err)
	assert.Equal(t, "orchestration second", secondDefinition.Description)

	secondActivity, err := store.FindActivityDefinition(ctx, "second", "test-activity")
	require.NoError(t, err)
	assert.Equal(t, "activity second", secondActivity.Description)

	_, err = store.FindOrchestrationDefinition(ctx, "third", "test.deploy")
	assert.Error(t, err)
	exists, err := store.ExistsActivityDefinition(ctx, "third", "test-activity")
	require.NoError(t, err)
	assert.False(t, exists)

	found, err := collection.CollectAll(store.FindOrchestrationDefinitionsByPredicate(ctx, "second", query.Eq("templateRef", "template-1")))
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "second", found[0].Namespace)

	activities, err := collection.CollectAll(store.FindActivityDefinitionsByPredicate(ctx, model.DefaultNamespace, query.Eq("type", "test-activity")))
	require.NoError(t, err)
	require.Len(t, activities, 1)
	assert.Equal(t, model.DefaultNamespace, activities[0].Namespace)

	orchestrations, err := store.ListOrchestrationDefinitions(ctx, "second")
	require.NoError(t, err)
	require.Len(t, orchestrations, 1)
	assert.Equal(t, "second", orchestrations[0].Namespace)

	listedActivities, err := store.ListActivityDefinitions(ctx, "third")
	require.NoError(t, err)
	assert.Empty(t, listedActivities)

	references, err := store.ActivityDefinitionReferences(ctx, "second", "test-activity")
	require.NoError(t, err)
	assert.Equal(t, []string{"test.deploy"}, references)
	references, err = store.ActivityDefinitionReferences(ctx, "third", "test-activity")
	require.NoError(t, err)
	assert.Empty(t, references)

	deleted, err := store.DeleteOrchestrationDefinition(ctx, "second", "test.deploy")
	require.NoError(t, err)
	assert.True(t, deleted)
	deleted, err = store.DeleteActivityDefinition(ctx, "second", "test-activity")
	require.NoError(t, err)
	assert.True(t, deleted)

	exists, err = store.ExistsOrchestrationDefinition(ctx, model.DefaultNamespace, "test.deploy")
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = store.ExistsActivityDefinition(ctx, "", "test-activity")
	require.NoError(t, err)
	assert.True(t, exists)
}
