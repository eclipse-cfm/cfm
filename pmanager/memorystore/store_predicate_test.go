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
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindOrchestrationDefinitionsByPredicate_EmptyStore
func TestFindOrchestrationDefinitionsByPredicate_EmptyStore(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	predicate := query.Eq("Active", true)

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 0, len(results))
}

// TestFindOrchestrationDefinitionsByPredicate_SingleMatch
func TestFindOrchestrationDefinitionsByPredicate_SingleMatch(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	var oType1 model.OrchestrationType = "deploy-orchestration"
	definition1 := &api.OrchestrationDefinition{
		Type:        oType1,
		Description: "Deploy",
		Active:      true,
	}
	_, _ = store.StoreOrchestrationDefinition(ctx, definition1)

	var oType2 model.OrchestrationType = "dispose-orchestration"
	definition2 := &api.OrchestrationDefinition{
		Type:        oType2,
		Description: "Dispose",
		Active:      false,
	}
	_, _ = store.StoreOrchestrationDefinition(ctx, definition2)

	predicate := query.Eq("Active", true)

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 1, len(results))
	assert.Equal(t, oType1, results[0].Type)
	assert.True(t, results[0].Active)
}

// TestFindOrchestrationDefinitionsByPredicate_MultipleMatches
func TestFindOrchestrationDefinitionsByPredicate_MultipleMatches(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		oType := model.OrchestrationType("orchestration-" + string(rune(48+i)))
		definition := &api.OrchestrationDefinition{
			Type:   oType,
			Active: i%2 == 0,
		}
		_, _ = store.StoreOrchestrationDefinition(ctx, definition)
	}

	predicate := query.Eq("Active", true)

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 2, len(results))
	for _, result := range results {
		assert.True(t, result.Active)
	}
}

// TestFindOrchestrationDefinitionsByPredicate_NoMatches
func TestFindOrchestrationDefinitionsByPredicate_NoMatches(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	var oType model.OrchestrationType = "test-orchestration"
	definition := &api.OrchestrationDefinition{
		Type:   oType,
		Active: false,
	}
	_, _ = store.StoreOrchestrationDefinition(ctx, definition)

	predicate := query.Eq("Active", true)

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 0, len(results))
}

// TestFindOrchestrationDefinitionsByPredicate_DataIsolation
func TestFindOrchestrationDefinitionsByPredicate_DataIsolation(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	var oType model.OrchestrationType = "test-orchestration"
	originalDef := &api.OrchestrationDefinition{
		Type:        oType,
		Description: "Test",
	}
	_, _ = store.StoreOrchestrationDefinition(ctx, originalDef)

	predicate := query.IsNotNull("Type")

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 1, len(results))

	results[0].Type = "modified-type"

	storedDef, err := store.FindOrchestrationDefinition(ctx, oType)
	require.NoError(t, err)
	assert.Equal(t, oType, storedDef.Type)
}

// TestFindActivityDefinitionsByPredicate_EmptyStore
func TestFindActivityDefinitionsByPredicate_EmptyStore(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	predicate := query.IsNotNull("Type")

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 0, len(results))
}

// TestFindActivityDefinitionsByPredicate_SingleMatch
func TestFindActivityDefinitionsByPredicate_SingleMatch(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	var actType1 api.ActivityType = "deploy-activity"
	definition1 := &api.ActivityDefinition{
		Type:        actType1,
		Description: "Deploy",
	}
	_, _ = store.StoreActivityDefinition(ctx, definition1)

	var actType2 api.ActivityType = "dispose-activity"
	definition2 := &api.ActivityDefinition{
		Type:        actType2,
		Description: "Dispose",
	}
	_, _ = store.StoreActivityDefinition(ctx, definition2)

	predicate := query.Contains("Description", "Deploy")

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 1, len(results))
	assert.Equal(t, actType1, results[0].Type)
	assert.Equal(t, "Deploy", results[0].Description)
}

// TestFindActivityDefinitionsByPredicate_MultipleMatches
func TestFindActivityDefinitionsByPredicate_MultipleMatches(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	for i := 1; i <= 4; i++ {
		actType := api.ActivityType("activity-" + string(rune(48+i)))
		definition := &api.ActivityDefinition{
			Type:        actType,
			Description: "Activity " + string(rune(48+i)),
		}
		_, _ = store.StoreActivityDefinition(ctx, definition)
	}

	predicate := query.IsNotNull("Type")

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 4, len(results))
}

// TestFindActivityDefinitionsByPredicate_NoMatches
func TestFindActivityDefinitionsByPredicate_NoMatches(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	var actType api.ActivityType = "test-activity"
	definition := &api.ActivityDefinition{
		Type:        actType,
		Description: "Test",
	}
	_, _ = store.StoreActivityDefinition(ctx, definition)

	predicate := query.Contains("Description", "NonExistent")

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 0, len(results))
}

// TestFindActivityDefinitionsByPredicate_DataIsolation
func TestFindActivityDefinitionsByPredicate_DataIsolation(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	var actType api.ActivityType = "test-activity"
	originalDef := &api.ActivityDefinition{
		Type:        actType,
		Description: "Test Activity",
	}
	_, _ = store.StoreActivityDefinition(ctx, originalDef)

	predicate := query.IsNotNull("Type")

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 1, len(results))

	results[0].Description = "modified-description"

	storedDef, err := store.FindActivityDefinition(ctx, actType)
	require.NoError(t, err)
	assert.Equal(t, "Test Activity", storedDef.Description)
}

// TestFindOrchestrationDefinitionsByPredicate_CompoundPredicate_AND
func TestFindOrchestrationDefinitionsByPredicate_CompoundPredicate_AND(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	defs := []struct {
		tpe    model.OrchestrationType
		desc   string
		active bool
	}{
		{"deploy", "Deploy orchestration", true},
		{"dispose", "Dispose orchestration", false},
		{"activate", "Activate orchestration", true},
		{"deactivate", "Deactivate orchestration", false},
	}

	for _, d := range defs {
		definition := &api.OrchestrationDefinition{
			Type:        d.tpe,
			Description: d.desc,
			Active:      d.active,
		}
		_, _ = store.StoreOrchestrationDefinition(ctx, definition)
	}

	// Predicate: Active=true AND Description contains "orchestration"
	predicate := query.And(
		query.Eq("Active", true),
		query.Contains("Description", "orchestration"),
	)

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 2, len(results))
	for _, result := range results {
		assert.True(t, result.Active)
		assert.Contains(t, result.Description, "orchestration")
	}
}

// TestFindOrchestrationDefinitionsByPredicate_CompoundPredicate_OR
func TestFindOrchestrationDefinitionsByPredicate_CompoundPredicate_OR(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	defs := []struct {
		tpe    model.OrchestrationType
		desc   string
		active bool
	}{
		{"deploy", "Deploy", true},
		{"dispose", "Dispose", false},
		{"activate", "Activate", true},
		{"deactivate", "Deactivate", false},
	}

	for _, d := range defs {
		definition := &api.OrchestrationDefinition{
			Type:        d.tpe,
			Description: d.desc,
			Active:      d.active,
		}
		_, _ = store.StoreOrchestrationDefinition(ctx, definition)
	}

	// Predicate: Type="deploy" OR Active=false
	predicate := query.Or(
		query.Eq("Type", model.OrchestrationType("deploy")),
		query.Eq("Active", false),
	)

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 3, len(results))
}

// TestFindActivityDefinitionsByPredicate_CompoundPredicate_AND
func TestFindActivityDefinitionsByPredicate_CompoundPredicate_AND(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	defs := []struct {
		tpe  api.ActivityType
		desc string
	}{
		{"create-resource", "Creates a resource"},
		{"delete-resource", "Deletes a resource"},
		{"configure", "Configures resources"},
		{"validate", "Validates configuration"},
	}

	for _, d := range defs {
		definition := &api.ActivityDefinition{
			Type:        d.tpe,
			Description: d.desc,
		}
		_, _ = store.StoreActivityDefinition(ctx, definition)
	}

	// Predicate: Type contains "resource" AND Description contains "Deletes"
	predicate := query.And(
		query.Contains("Type", "resource"),
		query.Contains("Description", "Deletes"),
	)

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 1, len(results))
	assert.Equal(t, api.ActivityType("delete-resource"), results[0].Type)
}

// TestFindActivityDefinitionsByPredicate_CompoundPredicate_OR
func TestFindActivityDefinitionsByPredicate_CompoundPredicate_OR(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	defs := []struct {
		tpe  api.ActivityType
		desc string
	}{
		{"create-resource", "Creates a resource"},
		{"delete-resource", "Deletes a resource"},
		{"configure", "Configures resources"},
		{"validate", "Validates configuration"},
	}

	for _, d := range defs {
		definition := &api.ActivityDefinition{
			Type:        d.tpe,
			Description: d.desc,
		}
		_, _ = store.StoreActivityDefinition(ctx, definition)
	}

	// Predicate: Type="configure" OR Type="validate"
	predicate := query.Or(
		query.Eq("Type", api.ActivityType("configure")),
		query.Eq("Type", api.ActivityType("validate")),
	)

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 2, len(results))
}

// TestFindOrchestrationDefinitionsByPredicate_In
func TestFindOrchestrationDefinitionsByPredicate_In(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		oType := model.OrchestrationType("orchestration-" + string(rune(48+i)))
		definition := &api.OrchestrationDefinition{
			Type: oType,
		}
		_, _ = store.StoreOrchestrationDefinition(ctx, definition)
	}

	predicate := query.In("Type",
		model.OrchestrationType("orchestration-1"),
		model.OrchestrationType("orchestration-3"),
	)

	results := make([]api.OrchestrationDefinition, 0)
	for def, err := range store.FindOrchestrationDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 2, len(results))
}

// TestFindActivityDefinitionsByPredicate_StartsWith
func TestFindActivityDefinitionsByPredicate_StartsWith(t *testing.T) {
	store := NewDefinitionStore()
	ctx := context.Background()

	defs := []api.ActivityType{"create-resource", "create-network", "delete-resource", "validate-config"}

	for _, d := range defs {
		definition := &api.ActivityDefinition{
			Type: d,
		}
		_, _ = store.StoreActivityDefinition(ctx, definition)
	}

	predicate := query.StartsWith("Type", "create")

	results := make([]api.ActivityDefinition, 0)
	for def, err := range store.FindActivityDefinitionsByPredicate(ctx, predicate) {
		require.NoError(t, err)
		results = append(results, def)
	}

	assert.Equal(t, 2, len(results))
}
