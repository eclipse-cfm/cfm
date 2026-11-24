//  Copyright (c) 2026 Metaform Systems, Inc
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
	"testing"

	"github.com/eclipse-cfm/cfm/common/collection"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresDefinitionStore_CompensationTypeRoundTrip(t *testing.T) {
	setupOrchestrationDefinitionTable(t, testDB)
	defer cleanupOrchestrationDefinitionTestData(t, testDB)

	store := newPostgresDefinitionStore()
	ctx := context.Background()
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	_, err = store.StoreOrchestrationDefinition(txCtx, &api.OrchestrationDefinition{
		Type:             "test.deploy",
		Version:          1,
		TemplateRef:      "template-1",
		CompensationType: "test.rollback",
	})
	require.NoError(t, err)
	_, err = store.StoreOrchestrationDefinition(txCtx, &api.OrchestrationDefinition{
		Type:        "test.rollback",
		Version:     1,
		TemplateRef: "template-1",
	})
	require.NoError(t, err)

	deploy, err := store.FindOrchestrationDefinition(txCtx, model.DefaultNamespace, "test.deploy")
	require.NoError(t, err)
	assert.Equal(t, model.OrchestrationType("test.rollback"), deploy.CompensationType)

	rollback, err := store.FindOrchestrationDefinition(txCtx, model.DefaultNamespace, "test.rollback")
	require.NoError(t, err)
	assert.Empty(t, rollback.CompensationType)

	var column sql.NullString
	require.NoError(t, tx.QueryRow("SELECT compensation_type FROM orchestration_definitions WHERE id = 'default/test.rollback'").Scan(&column))
	assert.False(t, column.Valid, "a definition without compensation stores NULL")

	found, err := collection.CollectAll(store.FindOrchestrationDefinitionsByPredicate(txCtx, model.DefaultNamespace, query.Eq("compensationType", "test.rollback")))
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, model.OrchestrationType("test.deploy"), found[0].Type)
}

func compensationTypeOf(t *testing.T, id string) string {
	t.Helper()
	var compensationType sql.NullString
	require.NoError(t, testDB.QueryRow("SELECT compensation_type FROM orchestration_definitions WHERE id = $1", id).Scan(&compensationType))
	return compensationType.String
}
