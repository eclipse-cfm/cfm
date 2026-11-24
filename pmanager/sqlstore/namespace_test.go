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
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNamespaceTestContext(t *testing.T) context.Context {
	t.Helper()
	setupActivityDefinitionTable(t, testDB)
	setupOrchestrationDefinitionTable(t, testDB)
	t.Cleanup(func() {
		cleanupActivityDefinitionTestData(t, testDB)
		cleanupOrchestrationDefinitionTestData(t, testDB)
	})
	tx, err := testDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return context.WithValue(context.Background(), sqlstore.SQLTransactionKey, tx)
}

func TestPostgresDefinitionStore_SameTypeInTwoNamespaces(t *testing.T) {
	ctx := newNamespaceTestContext(t)
	store := newPostgresDefinitionStore()

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
	assert.Equal(t, "second", secondDefinition.Namespace)
	assert.Equal(t, "orchestration second", secondDefinition.Description)

	secondActivity, err := store.FindActivityDefinition(ctx, "second", "test-activity")
	require.NoError(t, err)
	assert.Equal(t, "activity second", secondActivity.Description)

	_, err = store.FindOrchestrationDefinition(ctx, "third", "test.deploy")
	assert.Error(t, err)
	exists, err := store.ExistsActivityDefinition(ctx, "third", "test-activity")
	require.NoError(t, err)
	assert.False(t, exists)

	// queries and lists only return the definitions of the namespace
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

	// an update only changes the definition of its namespace
	secondDefinition.Description = "updated"
	_, err = store.StoreOrchestrationDefinition(ctx, secondDefinition)
	require.NoError(t, err)
	defaultDefinition, err = store.FindOrchestrationDefinition(ctx, "", "test.deploy")
	require.NoError(t, err)
	assert.Equal(t, "orchestration ", defaultDefinition.Description)

	// a delete only removes the definition of its namespace
	deleted, err := store.DeleteOrchestrationDefinition(ctx, "second", "test.deploy")
	require.NoError(t, err)
	assert.True(t, deleted)
	deleted, err = store.DeleteActivityDefinition(ctx, "second", "test-activity")
	require.NoError(t, err)
	assert.True(t, deleted)

	exists, err = store.ExistsOrchestrationDefinition(ctx, model.DefaultNamespace, "test.deploy")
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = store.ExistsActivityDefinition(ctx, model.DefaultNamespace, "test-activity")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCreateTables_MigratesDefinitionsToDefaultNamespace(t *testing.T) {
	cleanupActivityDefinitionTestData(t, testDB)
	cleanupOrchestrationDefinitionTestData(t, testDB)
	cleanupOrchestrationEntryTestData(t, testDB)
	t.Cleanup(func() {
		cleanupActivityDefinitionTestData(t, testDB)
		cleanupOrchestrationDefinitionTestData(t, testDB)
		cleanupOrchestrationEntryTestData(t, testDB)
	})

	// the tables and rows as they were stored before namespaces were introduced
	_, err := testDB.Exec(`
		CREATE TABLE activity_definitions (
			id VARCHAR(255) PRIMARY KEY, "type" VARCHAR(255), version BIGINT NOT NULL, description TEXT,
			input_schema JSONB, output_schema JSONB);
		CREATE TABLE orchestration_definitions (
			id VARCHAR(255) PRIMARY KEY, "type" VARCHAR(255), version BIGINT NOT NULL, description TEXT,
			active BOOLEAN DEFAULT FALSE, "schema" JSONB, activities JSONB, templateref TEXT, compensation_type VARCHAR(255));
		CREATE TABLE orchestration_entries (
			id VARCHAR(255) PRIMARY KEY, version BIGINT NOT NULL, correlation_id VARCHAR(255) NOT NULL,
			definition_id VARCHAR(255) NOT NULL, "state" INTEGER, state_timestamp TIMESTAMP NOT NULL,
			created_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, orchestration_type VARCHAR(255));
		INSERT INTO activity_definitions (id, "type", version, description) VALUES ('test-activity', 'test-activity', 1, 'legacy');
		INSERT INTO orchestration_definitions (id, "type", version, description, active, templateref, activities)
			VALUES ('test.deploy', 'test.deploy', 1, 'legacy', true, 'template-1', '[{"id":"a1","type":"test-activity"}]');
	`)
	require.NoError(t, err)
	_, err = testDB.Exec(`INSERT INTO orchestration_entries (id, version, correlation_id, definition_id, "state", state_timestamp, orchestration_type)
			VALUES ('o1', 1, 'c1', 'test.deploy', 2, $1, 'test.deploy')`, time.Now())
	require.NoError(t, err)

	// the migration runs on every start, so it must be idempotent
	require.NoError(t, createTables(testDB))
	require.NoError(t, createTables(testDB))

	ctx := context.WithValue(context.Background(), sqlstore.SQLTransactionKey, mustBegin(t))
	store := newPostgresDefinitionStore()

	definition, err := store.FindOrchestrationDefinition(ctx, model.DefaultNamespace, "test.deploy")
	require.NoError(t, err)
	assert.Equal(t, model.DefaultNamespace, definition.Namespace)
	assert.Equal(t, "legacy", definition.Description)

	activity, err := store.FindActivityDefinition(ctx, model.DefaultNamespace, "test-activity")
	require.NoError(t, err)
	assert.Equal(t, model.DefaultNamespace, activity.Namespace)

	// entries are keyed by namespace and orchestration ID
	entry, err := newOrchestrationEntryStore().FindByID(ctx, "default.o1")
	require.NoError(t, err)
	assert.Equal(t, model.DefaultNamespace, entry.Namespace)
	assert.Equal(t, "o1", entry.ID)
	assert.Equal(t, "test.deploy", entry.DefinitionID)

	var ids []string
	rows, err := testDB.Query("SELECT id FROM orchestration_definitions UNION ALL SELECT id FROM activity_definitions ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	assert.Equal(t, []string{"default/test-activity", "default/test.deploy"}, ids)
}

func TestOrchestrationEntryStore_Namespace(t *testing.T) {
	setupOrchestrationEntryTable(t, testDB)
	t.Cleanup(func() { cleanupOrchestrationEntryTestData(t, testDB) })
	ctx := context.WithValue(context.Background(), sqlstore.SQLTransactionKey, mustBegin(t))
	entries := newOrchestrationEntryStore()

	// the same orchestration ID in two namespaces makes two entries
	for _, entry := range []*api.OrchestrationEntry{
		{ID: "o1", CorrelationID: "c1", DefinitionID: "test.deploy", OrchestrationType: "test.deploy", StateTimestamp: time.Now(), CreatedTimestamp: time.Now()},
		{ID: "o1", Namespace: "second", CorrelationID: "c2", DefinitionID: "test.deploy", OrchestrationType: "test.deploy", StateTimestamp: time.Now(), CreatedTimestamp: time.Now()},
	} {
		_, err := entries.Create(ctx, entry)
		require.NoError(t, err)
	}

	found, err := collection.CollectAll(entries.FindByPredicate(ctx, query.Eq("namespace", "second")))
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "o1", found[0].ID)
	assert.Equal(t, "second", found[0].Namespace)
	assert.Equal(t, "c2", found[0].CorrelationID)

	// a predicate on the id is on the orchestration ID
	found, err = collection.CollectAll(entries.FindByPredicate(ctx, query.And(query.Eq("namespace", model.DefaultNamespace), query.Eq("id", "o1"))))
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "c1", found[0].CorrelationID)

	entry, err := entries.FindByID(ctx, "default.o1")
	require.NoError(t, err)
	assert.Equal(t, model.DefaultNamespace, entry.Namespace)
	assert.Equal(t, "o1", entry.ID)
	entry, err = entries.FindByID(ctx, api.OrchestrationKey("second", "o1"))
	require.NoError(t, err)
	assert.Equal(t, "second", entry.Namespace)
}

func mustBegin(t *testing.T) any {
	t.Helper()
	tx, err := testDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}
