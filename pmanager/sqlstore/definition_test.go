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

package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/lib/pq"
	"github.com/metaform/connector-fabric-manager/common/collection"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/sqlstore"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresDefinitionStore_FindOrchestrationDefinition tests finding an orchestration definition by type
func TestPostgresDefinitionStore_FindOrchestrationDefinition(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	definition := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("provision"),
		Version:     1,
		Description: "Test provisioning orchestration",
		Active:      true,
		TemplateRef: "template1",
	}

	_, err := testDB.Exec(
		"INSERT INTO orchestration_definitions (id, type, version, description, active,templateref) VALUES ($1, $2, $3, $4, $5, $6)",
		definition.Type,
		definition.Type,
		definition.Version,
		definition.Description,
		definition.Active,
		definition.TemplateRef,
	)
	require.NoError(t, err)

	store := newPostgresDefinitionStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	found, err := store.FindOrchestrationDefinition(txCtx, "provision")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, model.OrchestrationType("provision"), found.Type)
	assert.Equal(t, int64(1), found.Version)
	assert.Equal(t, "Test provisioning orchestration", found.Description)
	assert.Equal(t, true, found.Active)

	exists, err := store.ExistsOrchestrationDefinition(txCtx, "provision")
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestFindActivityDefinition_NotFound tests retrieval of non-existent orchestration definition
func TestFindOrchestrationDefinition_NotFound(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Test finding non-existent orchestration definition
	found, err := store.FindActivityDefinition(txCtx, "non-existent-orchestration")
	require.Error(t, err)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, types.ErrNotFound)
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_Type tests FindOrchestrationDefinitionsByPredicate with type predicate
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_Type(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions with different types
	def1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("workflow-v1"),
		Version:     1,
		Description: "First workflow",
		Active:      true,
		Activities:  []api.Activity{},
	}

	def2 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("process-v1"),
		Version:     1,
		Description: "First process",
		Active:      true,
		Activities:  []api.Activity{},
	}

	def3 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("workflow-v2"),
		Version:     2,
		Description: "Second workflow",
		Active:      false,
		Activities:  []api.Activity{},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("type", "workflow-v1")

	count := 0
	for definition, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.Equal(t, model.OrchestrationType("workflow-v1"), definition.Type)
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_Active tests FindOrchestrationDefinitionsByPredicate with active predicate
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_Active(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions with different active states
	def1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("active-def-1"),
		Version:     1,
		Description: "Active definition 1",
		Active:      true,
		Activities:  []api.Activity{},
	}

	def2 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("inactive-def-1"),
		Version:     1,
		Description: "Inactive definition 1",
		Active:      false,
		Activities:  []api.Activity{},
	}

	def3 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("active-def-2"),
		Version:     1,
		Description: "Active definition 2",
		Active:      true,
		Activities:  []api.Activity{},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("active", true)

	count := 0
	for definition, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.True(t, definition.Active)
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActiveFalse tests FindOrchestrationDefinitionsByPredicate with active=false predicate
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActiveFalse(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("active-def"),
		Version:     1,
		Description: "Active",
		Active:      true,
		Activities:  []api.Activity{},
	}

	def2 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("inactive-def-1"),
		Version:     1,
		Description: "Inactive 1",
		Active:      false,
		Activities:  []api.Activity{},
	}

	def3 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("inactive-def-2"),
		Version:     1,
		Description: "Inactive 2",
		Active:      false,
		Activities:  []api.Activity{},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("active", false)

	count := 0
	for definition, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.False(t, definition.Active)
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActivitiesType tests FindOrchestrationDefinitionsByPredicate with activities.type predicate
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActivitiesType(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions with different activities
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("with-email"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
			{Type: api.ActivityType("logging")},
		},
	}

	def2 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("with-sms"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("sms")},
		},
	}

	def3 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("with-logging"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("logging")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("activities.type", "dns-provision")

	count := 0
	for definition, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		// Verify at least one activity has type "dns-provision"
		found := false
		for _, activity := range definition.Activities {
			if activity.Type == ("dns-provision") {
				found = true
				break
			}
		}
		assert.True(t, found, "Definition should contain email activity")
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActivitiesType_Multiple tests querying for multiple activities.type matches
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActivitiesType_Multiple(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("logging-def-1"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("logging")},
			{Type: api.ActivityType("dns-provision")},
		},
	}

	def2 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("logging-def-2"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("logging")},
		},
	}

	def3 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("no-logging"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("sms")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("activities.type", "logging")

	count := 0
	for definition, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		// Verify at least one activity has type "logging"
		found := false
		for _, activity := range definition.Activities {
			if activity.Type == ("logging") {
				found = true
				break
			}
		}
		assert.True(t, found)
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_CompoundPredicate tests FindOrchestrationDefinitionsByPredicate with compound AND predicate
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_CompoundPredicate(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("active-email"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
		},
	}

	def2 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("inactive-email"),
		Version: 1,
		Active:  false,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
		},
	}

	def3 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("active-sms"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("sms")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.And(
		query.Eq("active", true),
		query.Eq("activities.type", "dns-provision"),
	)

	results, err := collection.CollectAll(store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate))
	require.NoError(t, err)
	assert.Len(t, results, 1)
	for definition, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.True(t, definition.Active)
		// Verify at least one activity has type "dns-provision"
		assert.Contains(t, definition.Activities, api.Activity{Type: "dns-provision"})
	}
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_NoMatches tests FindOrchestrationDefinitionsByPredicate with no matches
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_NoMatches(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("test-def"),
		Version:     1,
		Description: "Test definition",
		Active:      true,
		Activities:  []api.Activity{},
	}

	_, err = store.StoreOrchestrationDefinition(txCtx, def1)
	require.NoError(t, err)

	// Query for non-existent type
	predicate := query.Eq("type", "nonexistent")

	count := 0
	for _, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		count++
	}

	assert.Equal(t, 0, count)
}

// TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActivitiesType_NotFound tests activities.type predicate with no matches
func TestPostgresDefinitionStore_FindOrchestrationDefinitionsByPredicate_ActivitiesType_NotFound(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("no-notification"),
		Version:     1,
		Description: "Definition without notification activities",
		Active:      true,
		Activities: []api.Activity{
			{Type: api.ActivityType("logging")},
		},
	}

	_, err = store.StoreOrchestrationDefinition(txCtx, def1)
	require.NoError(t, err)

	// Query for activities.type = "dns-provision" (should find nothing)
	predicate := query.Eq("activities.type", "dns-provision")

	count := 0
	for _, err := range store.FindOrchestrationDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		count++
	}

	assert.Equal(t, 0, count)
}

// TestPostgresDefinitionStore_FindActivityDefinition tests finding an activity definition by type
func TestPostgresDefinitionStore_FindActivityDefinition(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	definition := &api.ActivityDefinition{
		Type:        api.ActivityType("dns-activation"),
		Version:     1,
		Description: "Test DNS activation activity",
	}

	inputSchema := map[string]any{"type": "string", "name": "domain"}
	outputSchema := map[string]any{"type": "string", "name": "status"}

	inputSchemaBytes, err := json.Marshal(inputSchema)
	require.NoError(t, err)

	outputSchemaBytes, err := json.Marshal(outputSchema)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO activity_definitions (id, type, version, description, input_schema, output_schema) VALUES ($1, $2, $3, $4, $5, $6)",
		definition.Type,
		definition.Type,
		definition.Version,
		definition.Description,
		inputSchemaBytes,
		outputSchemaBytes,
	)
	require.NoError(t, err)

	store := newPostgresDefinitionStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	found, err := store.FindActivityDefinition(txCtx, "dns-activation")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, api.ActivityType("dns-activation"), found.Type)
	assert.Equal(t, int64(1), found.Version)
	assert.Equal(t, "Test DNS activation activity", found.Description)
	assert.NotNil(t, found.InputSchema)
	assert.NotNil(t, found.OutputSchema)

	exists, err := store.ExistsActivityDefinition(txCtx, "dns-activation")
	require.NoError(t, err)
	assert.True(t, exists)

}

// TestFindActivityDefinition_NotFound tests retrieval of non-existent activity definition
func TestFindActivityDefinition_NotFound(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Test finding non-existent activity definition
	found, err := store.FindActivityDefinition(txCtx, "non-existent-activity")
	require.Error(t, err)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, types.ErrNotFound)
}

// TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_Type tests FindActivityDefinitionsByPredicate with type predicate
func TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_Type(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test activity definitions with different types
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("email-v1"),
		Version:     1,
		Description: "First email activity",
	}

	def2 := &api.ActivityDefinition{
		Type:        api.ActivityType("sms-v1"),
		Version:     1,
		Description: "First SMS activity",
	}

	def3 := &api.ActivityDefinition{
		Type:        api.ActivityType("email-v2"),
		Version:     2,
		Description: "Second email activity",
	}

	for _, def := range []*api.ActivityDefinition{def1, def2, def3} {
		_, err := store.StoreActivityDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("type", "email-v1")

	count := 0
	for definition, err := range store.FindActivityDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.Equal(t, api.ActivityType("email-v1"), definition.Type)
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_Version tests FindActivityDefinitionsByPredicate with version predicate
func TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_Version(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test activity definitions with different versions
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-def-1"),
		Version:     1,
		Description: "Activity definition version 1",
	}

	def2 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-def-2"),
		Version:     1,
		Description: "Activity definition version 1",
	}

	def3 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-def-3"),
		Version:     2,
		Description: "Activity definition version 2",
	}

	for _, def := range []*api.ActivityDefinition{def1, def2, def3} {
		_, err := store.StoreActivityDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("version", int64(1))

	count := 0
	for definition, err := range store.FindActivityDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.Equal(t, int64(1), definition.Version)
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_Description tests FindActivityDefinitionsByPredicate with description predicate
func TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_Description(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test activity definitions with different descriptions
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("notification-email"),
		Version:     1,
		Description: "Email notification activity",
	}

	def2 := &api.ActivityDefinition{
		Type:        api.ActivityType("notification-sms"),
		Version:     1,
		Description: "SMS notification activity",
	}

	def3 := &api.ActivityDefinition{
		Type:        api.ActivityType("logging-activity"),
		Version:     1,
		Description: "Logging activity",
	}

	for _, def := range []*api.ActivityDefinition{def1, def2, def3} {
		_, err := store.StoreActivityDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("description", "Email notification activity")

	count := 0
	for definition, err := range store.FindActivityDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.Equal(t, "Email notification activity", definition.Description)
		count++
	}

	assert.Equal(t, 1, count)
}

// TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_CompoundPredicate tests FindActivityDefinitionsByPredicate with compound AND predicate
func TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_CompoundPredicate(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test activity definitions
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("email-v1"),
		Version:     1,
		Description: "Email activity",
	}

	def2 := &api.ActivityDefinition{
		Type:        api.ActivityType("email-v2"),
		Version:     2,
		Description: "Email activity",
	}

	def3 := &api.ActivityDefinition{
		Type:        api.ActivityType("sms-v1"),
		Version:     1,
		Description: "SMS activity",
	}

	for _, def := range []*api.ActivityDefinition{def1, def2, def3} {
		_, err := store.StoreActivityDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.And(
		query.Eq("version", int64(1)),
		query.Eq("description", "Email activity"),
	)

	results, err := collection.CollectAll(store.FindActivityDefinitionsByPredicate(txCtx, predicate))
	require.NoError(t, err)
	assert.Len(t, results, 1)
	for definition, err := range store.FindActivityDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.Equal(t, int64(1), definition.Version)
		assert.Equal(t, "Email activity", definition.Description)
	}
}

// TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_NoMatches tests FindActivityDefinitionsByPredicate with no matches
func TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_NoMatches(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test activity definitions
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("test-activity"),
		Version:     1,
		Description: "Test activity",
	}

	_, err = store.StoreActivityDefinition(txCtx, def1)
	require.NoError(t, err)

	// Query for non-existent type
	predicate := query.Eq("type", "nonexistent-activity")

	count := 0
	for _, err := range store.FindActivityDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		count++
	}

	assert.Equal(t, 0, count)
}

// TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_MultipleMatches tests querying for multiple activity definition matches
func TestPostgresDefinitionStore_FindActivityDefinitionsByPredicate_MultipleMatches(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test activity definitions
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("notification-email-v1"),
		Version:     1,
		Description: "Email notification",
	}

	def2 := &api.ActivityDefinition{
		Type:        api.ActivityType("notification-email-v2"),
		Version:     1,
		Description: "Email notification",
	}

	def3 := &api.ActivityDefinition{
		Type:        api.ActivityType("notification-sms"),
		Version:     1,
		Description: "SMS notification",
	}

	for _, def := range []*api.ActivityDefinition{def1, def2, def3} {
		_, err := store.StoreActivityDefinition(txCtx, def)
		require.NoError(t, err)
	}

	predicate := query.Eq("description", "Email notification")

	count := 0
	for definition, err := range store.FindActivityDefinitionsByPredicate(txCtx, predicate) {
		require.NoError(t, err)
		assert.Equal(t, "Email notification", definition.Description)
		count++
	}

	assert.Equal(t, 2, count)
}

// TestPostgresDefinitionStore_DeleteOrchestrationDefinition tests successful deletion of an orchestration definition
func TestPostgresDefinitionStore_DeleteOrchestrationDefinition(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test orchestration definition
	definition := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("delete-test-orchestration"),
		Version:     1,
		Description: "Orchestration to be deleted",
		Active:      true,
		Activities:  []api.Activity{},
	}

	_, err = store.StoreOrchestrationDefinition(txCtx, definition)
	require.NoError(t, err)

	// Verify it exists before deletion
	exists, err := store.ExistsOrchestrationDefinition(txCtx, "delete-test-orchestration")
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete the orchestration definition
	deleted, err := store.DeleteOrchestrationDefinition(txCtx, "delete-test-orchestration")
	require.NoError(t, err)
	assert.True(t, deleted)

	// Verify it no longer exists after deletion
	exists, err = store.ExistsOrchestrationDefinition(txCtx, "delete-test-orchestration")
	require.NoError(t, err)
	assert.False(t, exists)

	// Verify FindOrchestrationDefinition returns error after deletion
	found, err := store.FindOrchestrationDefinition(txCtx, "delete-test-orchestration")
	require.Error(t, err)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, types.ErrNotFound)
}

// TestPostgresDefinitionStore_DeleteOrchestrationDefinition_NotFound tests deletion of non-existent orchestration definition
func TestPostgresDefinitionStore_DeleteOrchestrationDefinition_NotFound(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Attempt to delete non-existent orchestration definition
	deleted, err := store.DeleteOrchestrationDefinition(txCtx, "nonexistent-orchestration")
	require.NoError(t, err)
	assert.False(t, deleted)
}

// TestPostgresDefinitionStore_DeleteOrchestrationDefinition_Multiple tests deletion of one orchestration definition doesn't affect others
func TestPostgresDefinitionStore_DeleteOrchestrationDefinition_Multiple(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert multiple orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-1"),
		Version:     1,
		Description: "Orchestration 1",
		Active:      true,
		Activities:  []api.Activity{},
	}

	def2 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-2"),
		Version:     1,
		Description: "Orchestration 2",
		Active:      true,
		Activities:  []api.Activity{},
	}

	def3 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-3"),
		Version:     1,
		Description: "Orchestration 3",
		Active:      false,
		Activities:  []api.Activity{},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// Delete the second definition
	deleted, err := store.DeleteOrchestrationDefinition(txCtx, "orchestration-2")
	require.NoError(t, err)
	assert.True(t, deleted)

	// Verify first definition still exists
	exists, err := store.ExistsOrchestrationDefinition(txCtx, "orchestration-1")
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify deleted definition does not exist
	exists, err = store.ExistsOrchestrationDefinition(txCtx, "orchestration-2")
	require.NoError(t, err)
	assert.False(t, exists)

	// Verify third definition still exists
	exists, err = store.ExistsOrchestrationDefinition(txCtx, "orchestration-3")
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestPostgresDefinitionStore_DeleteActivityDefinition tests successful deletion of an activity definition
func TestPostgresDefinitionStore_DeleteActivityDefinition(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert test activity definition
	definition := &api.ActivityDefinition{
		Type:        api.ActivityType("delete-test-activity"),
		Version:     1,
		Description: "Activity to be deleted",
	}

	_, err = store.StoreActivityDefinition(txCtx, definition)
	require.NoError(t, err)

	// Verify it exists before deletion
	exists, err := store.ExistsActivityDefinition(txCtx, "delete-test-activity")
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete the activity definition
	deleted, err := store.DeleteActivityDefinition(txCtx, "delete-test-activity")
	require.NoError(t, err)
	assert.True(t, deleted)

	// Verify it no longer exists after deletion
	exists, err = store.ExistsActivityDefinition(txCtx, "delete-test-activity")
	require.NoError(t, err)
	assert.False(t, exists)

	// Verify FindActivityDefinition returns error after deletion
	found, err := store.FindActivityDefinition(txCtx, "delete-test-activity")
	require.Error(t, err)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, types.ErrNotFound)
}

// TestPostgresDefinitionStore_DeleteActivityDefinition_NotFound tests deletion of non-existent activity definition
func TestPostgresDefinitionStore_DeleteActivityDefinition_NotFound(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Attempt to delete non-existent activity definition
	deleted, err := store.DeleteActivityDefinition(txCtx, "nonexistent-activity")
	require.NoError(t, err)
	assert.False(t, deleted)
}

// TestPostgresDefinitionStore_DeleteActivityDefinition_Multiple tests deletion of one activity definition doesn't affect others
func TestPostgresDefinitionStore_DeleteActivityDefinition_Multiple(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert multiple activity definitions
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-1"),
		Version:     1,
		Description: "Activity 1",
	}

	def2 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-2"),
		Version:     1,
		Description: "Activity 2",
	}

	def3 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-3"),
		Version:     2,
		Description: "Activity 3",
	}

	for _, def := range []*api.ActivityDefinition{def1, def2, def3} {
		_, err := store.StoreActivityDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// Delete the second definition
	deleted, err := store.DeleteActivityDefinition(txCtx, "activity-2")
	require.NoError(t, err)
	assert.True(t, deleted)

	// Verify first definition still exists
	exists, err := store.ExistsActivityDefinition(txCtx, "activity-1")
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify deleted definition does not exist
	exists, err = store.ExistsActivityDefinition(txCtx, "activity-2")
	require.NoError(t, err)
	assert.False(t, exists)

	// Verify third definition still exists
	exists, err = store.ExistsActivityDefinition(txCtx, "activity-3")
	require.NoError(t, err)
	assert.True(t, exists)
}

func setupActivityDefinitionTable(t *testing.T, db *sql.DB) {
	err := createActivityDefinitionsTable(db)
	require.NoError(t, err)
}

// TestPostgresDefinitionStore_ListOrchestrationDefinitions tests listing all orchestration definitions
func TestPostgresDefinitionStore_ListOrchestrationDefinitions(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert multiple orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-list-1"),
		Version:     1,
		Description: "Orchestration 1",
		Active:      true,
		Activities:  []api.Activity{},
	}

	def2 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-list-2"),
		Version:     2,
		Description: "Orchestration 2",
		Active:      false,
		Activities:  []api.Activity{},
	}

	def3 := &api.OrchestrationDefinition{
		Type:        model.OrchestrationType("orchestration-list-3"),
		Version:     1,
		Description: "Orchestration 3",
		Active:      true,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// List all orchestration definitions
	definitions, err := store.ListOrchestrationDefinitions(txCtx)
	require.NoError(t, err)
	assert.Len(t, definitions, 3)

	// Verify all definitions are present
	aTypes := make(map[string]bool)
	for _, def := range definitions {
		aTypes[def.Type.String()] = true
	}

	assert.True(t, aTypes["orchestration-list-1"])
	assert.True(t, aTypes["orchestration-list-2"])
	assert.True(t, aTypes["orchestration-list-3"])

	// Verify definition details
	for _, def := range definitions {
		switch def.Type {
		case "orchestration-list-1":
			assert.Equal(t, int64(1), def.Version)
			assert.Equal(t, "Orchestration 1", def.Description)
			assert.True(t, def.Active)
		case "orchestration-list-2":
			assert.Equal(t, int64(2), def.Version)
			assert.Equal(t, "Orchestration 2", def.Description)
			assert.False(t, def.Active)
		case "orchestration-list-3":
			assert.Equal(t, int64(1), def.Version)
			assert.Equal(t, "Orchestration 3", def.Description)
			assert.True(t, def.Active)
			assert.Len(t, def.Activities, 1)
		}
	}
}

// TestPostgresDefinitionStore_ListOrchestrationDefinitions_Empty tests listing orchestration definitions when none exist
func TestPostgresDefinitionStore_ListOrchestrationDefinitions_Empty(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// List orchestration definitions when none exist
	definitions, err := store.ListOrchestrationDefinitions(txCtx)
	require.NoError(t, err)
	assert.Empty(t, definitions)
}

// TestPostgresDefinitionStore_ListActivityDefinitions tests listing all activity definitions
func TestPostgresDefinitionStore_ListActivityDefinitions(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert multiple activity definitions
	def1 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-list-1"),
		Version:     1,
		Description: "Email activity",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"dns-provision": map[string]any{"type": "string"}}},
	}

	def2 := &api.ActivityDefinition{
		Type:         api.ActivityType("activity-list-2"),
		Version:      2,
		Description:  "SMS activity",
		OutputSchema: map[string]any{"type": "object", "properties": map[string]any{"status": map[string]any{"type": "string"}}},
	}

	def3 := &api.ActivityDefinition{
		Type:        api.ActivityType("activity-list-3"),
		Version:     1,
		Description: "Logging activity",
	}

	for _, def := range []*api.ActivityDefinition{def1, def2, def3} {
		_, err := store.StoreActivityDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// List all activity definitions
	definitions, err := store.ListActivityDefinitions(txCtx)
	require.NoError(t, err)
	assert.Len(t, definitions, 3)

	// Verify all definitions are present
	aTypes := make(map[string]bool)
	for _, def := range definitions {
		aTypes[def.Type.String()] = true
	}

	assert.True(t, aTypes["activity-list-1"])
	assert.True(t, aTypes["activity-list-2"])
	assert.True(t, aTypes["activity-list-3"])

	// Verify definition details
	for _, def := range definitions {
		switch def.Type {
		case "activity-list-1":
			assert.Equal(t, int64(1), def.Version)
			assert.Equal(t, "Email activity", def.Description)
			assert.NotNil(t, def.InputSchema)
		case "activity-list-2":
			assert.Equal(t, int64(2), def.Version)
			assert.Equal(t, "SMS activity", def.Description)
			assert.NotNil(t, def.OutputSchema)
		case "activity-list-3":
			assert.Equal(t, int64(1), def.Version)
			assert.Equal(t, "Logging activity", def.Description)
		}
	}
}

// TestPostgresDefinitionStore_ListActivityDefinitions_Empty tests listing activity definitions when none exist
func TestPostgresDefinitionStore_ListActivityDefinitions_Empty(t *testing.T) {
	setupActivityDefinitionTable(t, testDB)
	defer cleanupActivityDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// List activity definitions when none exist
	definitions, err := store.ListActivityDefinitions(txCtx)
	require.NoError(t, err)
	assert.Empty(t, definitions)
}

// TestPostgresDefinitionStore_ActivityDefinitionReferences tests finding orchestrations that reference an activity
func TestPostgresDefinitionStore_ActivityDefinitionReferences(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert orchestration definitions that reference the email activity
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("orchestration-with-email-1"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
			{Type: api.ActivityType("logging")},
		},
	}

	def2 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("orchestration-with-email-2"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
		},
	}

	def3 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("orchestration-with-sms"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("sms")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2, def3} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// Find all orchestrations that reference the email activity
	references, err := store.ActivityDefinitionReferences(txCtx, "dns-provision")
	require.NoError(t, err)
	assert.Len(t, references, 2)

	// Verify the orchestration types are in the references
	refMap := make(map[string]bool)
	for _, ref := range references {
		refMap[ref] = true
	}

	assert.True(t, refMap["orchestration-with-email-1"])
	assert.True(t, refMap["orchestration-with-email-2"])
	assert.False(t, refMap["orchestration-with-sms"])
}

// TestPostgresDefinitionStore_ActivityDefinitionReferences_SingleReference tests finding a single orchestration that references an activity
func TestPostgresDefinitionStore_ActivityDefinitionReferences_SingleReference(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert orchestration definitions
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("orchestration-sms-only"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("sms")},
		},
	}

	def2 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("orchestration-logging-only"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("logging")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// Find orchestrations that reference the sms activity
	references, err := store.ActivityDefinitionReferences(txCtx, "sms")
	require.NoError(t, err)
	assert.Len(t, references, 1)
	assert.Equal(t, "orchestration-sms-only", references[0])
}

// TestPostgresDefinitionStore_ActivityDefinitionReferences_NoReferences tests activity with no references
func TestPostgresDefinitionStore_ActivityDefinitionReferences_NoReferences(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert orchestration definitions without the email activity
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("orchestration-sms"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("sms")},
		},
	}

	def2 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("orchestration-logging"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("logging")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// Find orchestrations that reference the email activity (should find nothing)
	references, err := store.ActivityDefinitionReferences(txCtx, "dns-provision")
	require.NoError(t, err)
	assert.Empty(t, references)
}

// TestPostgresDefinitionStore_ActivityDefinitionReferences_EmptyOrchestrations tests activity references when no orchestrations exist
func TestPostgresDefinitionStore_ActivityDefinitionReferences_EmptyOrchestrations(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Find references when no orchestrations exist
	references, err := store.ActivityDefinitionReferences(txCtx, "dns-provision")
	require.NoError(t, err)
	assert.Empty(t, references)
}

// TestPostgresDefinitionStore_ActivityDefinitionReferences_MultipleActivities tests finding references with multiple activity types
func TestPostgresDefinitionStore_ActivityDefinitionReferences_MultipleActivities(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert orchestration with multiple activities
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("multi-activity-orchestration"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
			{Type: api.ActivityType("sms")},
			{Type: api.ActivityType("logging")},
			{Type: api.ActivityType("webhook")},
		},
	}

	_, err = store.StoreOrchestrationDefinition(txCtx, def1)
	require.NoError(t, err)

	// Verify the orchestration is found for each activity type
	for _, activityType := range []string{"dns-provision", "sms", "logging", "webhook"} {
		references, err := store.ActivityDefinitionReferences(txCtx, api.ActivityType(activityType))
		require.NoError(t, err)
		assert.Len(t, references, 1)
		assert.Equal(t, "multi-activity-orchestration", references[0])
	}
}

// TestPostgresDefinitionStore_ActivityDefinitionReferences_ActiveAndInactive tests finding references across active and inactive orchestrations
func TestPostgresDefinitionStore_ActivityDefinitionReferences_ActiveAndInactive(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()

	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Insert both active and inactive orchestrations that reference email
	def1 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("active-email-orchestration"),
		Version: 1,
		Active:  true,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
		},
	}

	def2 := &api.OrchestrationDefinition{
		Type:    model.OrchestrationType("inactive-dns-orchestration"),
		Version: 1,
		Active:  false,
		Activities: []api.Activity{
			{Type: api.ActivityType("dns-provision")},
		},
	}

	for _, def := range []*api.OrchestrationDefinition{def1, def2} {
		_, err := store.StoreOrchestrationDefinition(txCtx, def)
		require.NoError(t, err)
	}

	// Find references - should include both active and inactive
	references, err := store.ActivityDefinitionReferences(txCtx, "dns-provision")
	require.NoError(t, err)
	assert.Len(t, references, 2)

	refMap := make(map[string]bool)
	for _, ref := range references {
		refMap[ref] = true
	}

	assert.True(t, refMap["active-email-orchestration"])
	assert.True(t, refMap["inactive-dns-orchestration"])
}

func cleanupActivityDefinitionTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DROP TABLE IF EXISTS activity_definitions CASCADE")
	require.NoError(t, err)
}

func setupOrchestrationDefinitionTable(t *testing.T, db *sql.DB) {
	err := createOrchestrationDefinitionsTable(db)
	require.NoError(t, err)
}

func cleanupOrchestrationDefinitionTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DROP TABLE IF EXISTS orchestration_definitions CASCADE")
	require.NoError(t, err)
}
