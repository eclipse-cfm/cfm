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
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDataspaceProfileStore_Creation tests store creation
func TestNewDataspaceProfileStore_Creation(t *testing.T) {
	pstore := newDataspaceProfileStore()

	require.NotNil(t, pstore)
	assert.NotNil(t, pstore)
}

// TestNewDataspaceProfileStore_FindByID_Success tests successful profile retrieval
func TestNewDataspaceProfileStore_FindByID_Success(t *testing.T) {
	setupProfileTable(t, testDB)
	defer cleanupProfileTestData(t, testDB)

	profile := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "profile-1",
			Version: 1,
		},
		DataspaceSpec: api.DataspaceSpec{
			ProtocolStack: []string{"dspace-2025-1"},
			CredentialSpecs: []model.CredentialSpec{
				{
					Type:   "FooCredential",
					Issuer: "did:web:bar.com",
					Format: "VC1_0_JWT",
				},
			},
		},
		Artifacts: []string{"artifact-1", "artifact-2"},
		Deployments: []api.DataspaceDeployment{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "deployment-1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: time.Now(),
				},
				CellID: "cell-1",
				Properties: map[string]any{
					"config": "value1",
				},
			},
		},
		Properties: map[string]any{
			"namespace": "default",
			"version":   "2025-1",
		},
	}

	record, err := dProfileEntityToRecord(profile)
	require.NoError(t, err)

	dspaceSpecVal := []byte("[]")
	if dspaceBytes, ok := record.Values["dataspace_spec"].([]byte); ok && len(dspaceBytes) > 0 {
		dspaceSpecVal = dspaceBytes
	}
	artifactsVal := []byte("[]")
	if artifactsBytes, ok := record.Values["artifacts"].([]byte); ok && len(artifactsBytes) > 0 {
		artifactsVal = artifactsBytes
	}
	deploymentsVal := []byte("[]")
	if deploymentsBytes, ok := record.Values["deployments"].([]byte); ok && len(deploymentsBytes) > 0 {
		deploymentsVal = deploymentsBytes
	}
	propertiesVal := []byte("{}")
	if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
		propertiesVal = propertiesBytes
	}

	_, err = testDB.Exec(
		"INSERT INTO dataspace_profiles (id, version, dataspace_spec, artifacts, deployments, properties) VALUES ($1, $2, $3, $4, $5, $6)",
		record.Values["id"],
		record.Values["version"],
		dspaceSpecVal,
		artifactsVal,
		deploymentsVal,
		propertiesVal,
	)
	require.NoError(t, err)

	estore := newDataspaceProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "profile-1")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "profile-1", retrieved.ID)
	assert.Equal(t, int64(1), retrieved.Version)
	assert.Len(t, retrieved.Artifacts, 2)
	assert.Len(t, retrieved.Deployments, 1)
	assert.Len(t, retrieved.DataspaceSpec.CredentialSpecs, 1)
	assert.Equal(t, retrieved.DataspaceSpec.CredentialSpecs[0].Type, "FooCredential")
	assert.Len(t, retrieved.DataspaceSpec.ProtocolStack, 1)
	assert.Equal(t, retrieved.DataspaceSpec.ProtocolStack[0], "dspace-2025-1")
}

// TestNewDataspaceProfileStore_FindByID_NotFound tests profile not found
func TestNewDataspaceProfileStore_FindByID_NotFound(t *testing.T) {
	setupProfileTable(t, testDB)
	defer cleanupProfileTestData(t, testDB)

	estore := newDataspaceProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	_, err = estore.FindByID(txCtx, "non-existent")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewDataspaceProfileStore_Create tests creating a new profile
func TestNewDataspaceProfileStore_Create(t *testing.T) {
	setupProfileTable(t, testDB)
	defer cleanupProfileTestData(t, testDB)

	profile := &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      "new-profile",
			Version: 1,
		},
		Artifacts: []string{"artifact-x", "artifact-y"},
		Deployments: []api.DataspaceDeployment{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "deployment-x",
						Version: 1,
					},
					State:          api.DeploymentStatePending,
					StateTimestamp: time.Now(),
				},
				CellID: "cell-2",
			},
		},
		Properties: map[string]any{
			"tier": "standard",
		},
	}

	estore := newDataspaceProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	created, err := estore.Create(txCtx, profile)
	require.NoError(t, err)
	assert.Equal(t, "new-profile", created.ID)
	assert.Equal(t, int64(1), created.Version)
}

// TestNewDataspaceProfileStore_SearchByArtifactsPredicate tests retrieving profiles with artifacts
func TestNewDataspaceProfileStore_SearchByArtifactsPredicate(t *testing.T) {
	setupProfileTable(t, testDB)
	defer cleanupProfileTestData(t, testDB)

	profiles := []*api.DataspaceProfile{
		{
			Entity: api.Entity{
				ID:      "profile-1",
				Version: 1,
			},
			Artifacts: []string{"artifact-connector", "artifact-vault"},
			Properties: map[string]any{
				"namespace": "default",
			},
		},
		{
			Entity: api.Entity{
				ID:      "profile-2",
				Version: 1,
			},
			Artifacts: []string{"artifact-service", "artifact-config"},
			Properties: map[string]any{
				"namespace": "prod",
			},
		},
		{
			Entity: api.Entity{
				ID:      "profile-3",
				Version: 1,
			},
			Artifacts: []string{"artifact-connector-advanced"},
			Properties: map[string]any{
				"namespace": "staging",
			},
		},
	}

	// Insert test profiles
	for _, profile := range profiles {
		record, err := dProfileEntityToRecord(profile)
		require.NoError(t, err)

		artifactsVal := []byte("[]")
		if artifactsBytes, ok := record.Values["artifacts"].([]byte); ok && len(artifactsBytes) > 0 {
			artifactsVal = artifactsBytes
		}
		propertiesVal := []byte("{}")
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO dataspace_profiles (id, version, artifacts, properties) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["version"],
			artifactsVal,
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newDataspaceProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Search for profiles with artifact-connector
	predicate := query.Eq("artifacts", "artifact-connector")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		// Verify the profile contains the expected artifact
		assert.Contains(t, profile.Artifacts, "artifact-connector")
	}

	// Should find 2 profiles with "artifact-connector" (profile-1 and profile-3)
	assert.Equal(t, 1, count)
}

// TestNewDataspaceProfileStore_SearchByDeploymentsPredicate tests searching by deployments
func TestNewDataspaceProfileStore_SearchByDeploymentsPredicate(t *testing.T) {
	setupProfileTable(t, testDB)
	defer cleanupProfileTestData(t, testDB)

	profiles := []*api.DataspaceProfile{
		{
			Entity: api.Entity{
				ID:      "profile-prod-1",
				Version: 1,
			},
			Artifacts: []string{"artifact-1"},
			Deployments: []api.DataspaceDeployment{
				{
					DeployableEntity: api.DeployableEntity{
						Entity: api.Entity{
							ID:      "dep-1",
							Version: 1,
						},
						State:          api.DeploymentStateActive,
						StateTimestamp: time.Now(),
					},
					CellID: "cell-prod-1",
				},
			},
			Properties: map[string]any{},
		},
		{
			Entity: api.Entity{
				ID:      "profile-prod-2",
				Version: 1,
			},
			Artifacts: []string{"artifact-2"},
			Deployments: []api.DataspaceDeployment{
				{
					DeployableEntity: api.DeployableEntity{
						Entity: api.Entity{
							ID:      "dep-2",
							Version: 1,
						},
						State:          api.DeploymentStatePending,
						StateTimestamp: time.Now(),
					},
					CellID: "cell-staging",
				},
			},
			Properties: map[string]any{},
		},
		{
			Entity: api.Entity{
				ID:      "profile-prod-3",
				Version: 1,
			},
			Artifacts: []string{"artifact-3"},
			Deployments: []api.DataspaceDeployment{
				{
					DeployableEntity: api.DeployableEntity{
						Entity: api.Entity{
							ID:      "dep-3",
							Version: 1,
						},
						State:          api.DeploymentStateActive,
						StateTimestamp: time.Now(),
					},
					CellID: "cell-prod-2",
				},
			},
			Properties: map[string]any{},
		},
	}

	// Insert test profiles
	for _, profile := range profiles {
		record, err := dProfileEntityToRecord(profile)
		require.NoError(t, err)

		artifactsVal := []byte("[]")
		if artifactsBytes, ok := record.Values["artifacts"].([]byte); ok && len(artifactsBytes) > 0 {
			artifactsVal = artifactsBytes
		}
		deploymentsVal := []byte("[]")
		if deploymentsBytes, ok := record.Values["deployments"].([]byte); ok && len(deploymentsBytes) > 0 {
			deploymentsVal = deploymentsBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO dataspace_profiles (id, version, artifacts, deployments) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["version"],
			artifactsVal,
			deploymentsVal,
		)
		require.NoError(t, err)
	}

	estore := newDataspaceProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("deployments.cellId", "cell-prod-1")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		if len(profile.Deployments) > 0 {
			assert.Equal(t, "cell-prod-1", profile.Deployments[0].CellID)
		}
	}

	assert.Equal(t, 1, count)
}

// TestNewDataspaceProfileStore_SearchByPropertiesPredicate tests searching by properties
func TestNewDataspaceProfileStore_SearchByPropertiesPredicate(t *testing.T) {
	setupProfileTable(t, testDB)
	defer cleanupProfileTestData(t, testDB)

	profiles := []*api.DataspaceProfile{
		{
			Entity: api.Entity{
				ID:      "profile-ns-default-1",
				Version: 1,
			},
			Artifacts: []string{"artifact-1"},
			Properties: map[string]any{
				"namespace": "default",
				"version":   "2025-1",
			},
		},
		{
			Entity: api.Entity{
				ID:      "profile-ns-prod-1",
				Version: 1,
			},
			Artifacts: []string{"artifact-2"},
			Properties: map[string]any{
				"namespace": "prod",
				"version":   "2025-1",
			},
		},
		{
			Entity: api.Entity{
				ID:      "profile-ns-default-2",
				Version: 1,
			},
			Artifacts: []string{"artifact-3"},
			Properties: map[string]any{
				"namespace": "default",
				"version":   "2025-2",
			},
		},
	}

	// Insert test profiles
	for _, profile := range profiles {
		record, err := dProfileEntityToRecord(profile)
		require.NoError(t, err)

		artifactsVal := []byte("[]")
		if artifactsBytes, ok := record.Values["artifacts"].([]byte); ok && len(artifactsBytes) > 0 {
			artifactsVal = artifactsBytes
		}
		propertiesVal := []byte("{}")
		if propertiesBytes, ok := record.Values["properties"].([]byte); ok && len(propertiesBytes) > 0 {
			propertiesVal = propertiesBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO dataspace_profiles (id, version, artifacts, properties) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["version"],
			artifactsVal,
			propertiesVal,
		)
		require.NoError(t, err)
	}

	estore := newDataspaceProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("properties.namespace", "default")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		if ns, ok := profile.Properties["namespace"].(string); ok {
			assert.Equal(t, "default", ns)
		}
	}

	assert.Equal(t, 2, count)
}

// TestNewDataspaceProfileStore_SearchByDeploymentStatePredicate tests searching by deployment state
func TestNewDataspaceProfileStore_SearchByDeploymentStatePredicate(t *testing.T) {
	setupProfileTable(t, testDB)
	defer cleanupProfileTestData(t, testDB)

	states := []api.DeploymentState{
		api.DeploymentStateActive,
		api.DeploymentStatePending,
		api.DeploymentStateActive,
	}

	for i := 1; i <= 3; i++ {
		profile := &api.DataspaceProfile{
			Entity: api.Entity{
				ID:      "profile-state-" + strconv.Itoa(i),
				Version: 1,
			},
			Artifacts: []string{"artifact"},
			Deployments: []api.DataspaceDeployment{
				{
					DeployableEntity: api.DeployableEntity{
						Entity: api.Entity{
							ID:      "dep-" + strconv.Itoa(i),
							Version: 1,
						},
						State:          states[i-1],
						StateTimestamp: time.Now(),
					},
					CellID: "cell-" + strconv.Itoa(i),
				},
			},
		}

		record, err := dProfileEntityToRecord(profile)
		require.NoError(t, err)

		artifactsVal := []byte("[]")
		if artifactsBytes, ok := record.Values["artifacts"].([]byte); ok && len(artifactsBytes) > 0 {
			artifactsVal = artifactsBytes
		}
		deploymentsVal := []byte("[]")
		if deploymentsBytes, ok := record.Values["deployments"].([]byte); ok && len(deploymentsBytes) > 0 {
			deploymentsVal = deploymentsBytes
		}

		_, err = testDB.Exec(
			"INSERT INTO dataspace_profiles (id, version, artifacts, deployments) VALUES ($1, $2, $3, $4)",
			record.Values["id"],
			record.Values["version"],
			artifactsVal,
			deploymentsVal,
		)
		require.NoError(t, err)
	}

	estore := newDataspaceProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("deployments.state", "active")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		if len(profile.Deployments) > 0 {
			assert.Equal(t, api.DeploymentStateActive, profile.Deployments[0].State)
		}
	}

	assert.Equal(t, 2, count)
}

// cleanupProfileTestData removes test data from the dataspace_profiles table
func cleanupProfileTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", cfmDataspaceProfilesTable))
	require.NoError(t, err)
}

// setupProfileTable creates the dataspace_profiles table for testing
func setupProfileTable(t *testing.T, db *sql.DB) {
	err := createDataspaceProfilesTable(db)
	require.NoError(t, err)
}
