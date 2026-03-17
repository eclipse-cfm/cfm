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
	"time"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewParticipantProfileStore_Creation tests store creation
func TestNewParticipantProfileStore_Creation(t *testing.T) {
	pstore := newParticipantProfileStore()

	require.NotNil(t, pstore)
	assert.NotNil(t, pstore)
}

// TestNewParticipantProfileStore_FindByID_Success tests successful profile retrieval
func TestNewParticipantProfileStore_FindByID_Success(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profile := &api.ParticipantProfile{
		Entity: api.Entity{
			ID:      "pprofile-1",
			Version: 1,
		},
		Identifier:          "acme-corp",
		TenantID:            "tenant-1",
		DataspaceProfileIDs: []string{"dprofile-1", "dprofile-2"},
		ParticipantRoles:    map[string][]string{"dspace1": []string{"MembershipCredential"}},
		VPAs: []api.VirtualParticipantAgent{
			{
				DeployableEntity: api.DeployableEntity{
					Entity: api.Entity{
						ID:      "vpa-1",
						Version: 1,
					},
					State:          api.DeploymentStateActive,
					StateTimestamp: time.Now(),
				},
				Type: "connector",
				Properties: map[string]any{
					"endpoint": "http://connector:8080",
				},
			},
		},
		Properties: map[string]any{
			"region": "us-east-1",
			"tier":   "premium",
		},
		Error:       false,
		ErrorDetail: "",
	}

	rolesJSON, err := json.Marshal(profile.ParticipantRoles)
	require.NoError(t, err)
	dsProfileIdsJSON, err := json.Marshal(profile.DataspaceProfileIDs)
	require.NoError(t, err)
	vpasJSON, err := json.Marshal(profile.VPAs)
	require.NoError(t, err)
	propertiesJSON, err := json.Marshal(profile.Properties)
	require.NoError(t, err)

	_, err = testDB.Exec(
		"INSERT INTO participant_profiles (id, version, identifier, tenant_id, participant_roles, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
		profile.ID,
		profile.Version,
		profile.Identifier,
		profile.TenantID,
		rolesJSON,
		dsProfileIdsJSON,
		vpasJSON,
		profile.Error,
		profile.ErrorDetail,
		propertiesJSON,
	)
	require.NoError(t, err)

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	retrieved, err := estore.FindByID(txCtx, "pprofile-1")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "pprofile-1", retrieved.ID)
	assert.Equal(t, int64(1), retrieved.Version)
	assert.Equal(t, "acme-corp", retrieved.Identifier)
	assert.Equal(t, "tenant-1", retrieved.TenantID)
	assert.Equal(t, "MembershipCredential", retrieved.ParticipantRoles["dspace1"][0])
	assert.Len(t, retrieved.DataspaceProfileIDs, 2)
}

// TestNewParticipantProfileStore_FindByID_NotFound tests profile not found
func TestNewParticipantProfileStore_FindByID_NotFound(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	_, err = estore.FindByID(txCtx, "non-existent")
	require.Error(t, err)
	assert.ErrorAs(t, types.ErrNotFound, &err)
}

// TestNewParticipantProfileStore_Create tests creating a new profile
func TestNewParticipantProfileStore_Create(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profile := &api.ParticipantProfile{
		Entity: api.Entity{
			ID:      "pprofile-new",
			Version: 1,
		},
		Identifier:          "new-corp",
		TenantID:            "tenant-2",
		DataspaceProfileIDs: []string{"dprofile-3"},
		VPAs:                []api.VirtualParticipantAgent{},
		Properties: map[string]any{
			"environment": "production",
		},
		Error:       false,
		ErrorDetail: "",
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	created, err := estore.Create(txCtx, profile)
	require.NoError(t, err)
	assert.Equal(t, "pprofile-new", created.ID)
	assert.Equal(t, int64(1), created.Version)
	assert.Equal(t, "new-corp", created.Identifier)
}

// TestNewParticipantProfileStore_SearchByDataspaceProfileIDsPredicate tests filtering by dataspaceProfileIds
func TestNewParticipantProfileStore_SearchByDataspaceProfileIDsPredicate(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-1",
				Version: 1,
			},
			Identifier:          "corp-a",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{"dprofile-1", "dprofile-2"},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-2",
				Version: 1,
			},
			Identifier:          "corp-b",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{"dprofile-1", "dprofile-3"},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-3",
				Version: 1,
			},
			Identifier:          "corp-c",
			TenantID:            "tenant-2",
			DataspaceProfileIDs: []string{"dprofile-4"},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
	}

	for _, profile := range profiles {
		dsProfileIdsJSON, err := json.Marshal(profile.DataspaceProfileIDs)
		require.NoError(t, err)

		_, err = testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			dsProfileIdsJSON,
			[]byte("[]"),
			profile.Error,
			profile.ErrorDetail,
			[]byte("{}"),
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("dataspaceProfileIds", "dprofile-1")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		assert.Contains(t, profile.DataspaceProfileIDs, "dprofile-1")
	}

	assert.Equal(t, 2, count)
}

// TestNewParticipantProfileStore_SearchByVPAsPredicate tests filtering by VPAs
func TestNewParticipantProfileStore_SearchByVPAsPredicate(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-vpa-1",
				Version: 1,
			},
			Identifier:          "corp-vpa-a",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs: []api.VirtualParticipantAgent{
				{
					DeployableEntity: api.DeployableEntity{
						Entity: api.Entity{
							ID:      "vpa-1",
							Version: 1,
						},
						State:          api.DeploymentStateActive,
						StateTimestamp: time.Now(),
					},
					Type: "connector",
				},
				{
					DeployableEntity: api.DeployableEntity{
						Entity: api.Entity{
							ID:      "vpa-2",
							Version: 1,
						},
						State:          api.DeploymentStatePending,
						StateTimestamp: time.Now(),
					},
					Type: "credentialService",
				},
			},
			Properties: map[string]any{},
			Error:      false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-vpa-2",
				Version: 1,
			},
			Identifier:          "corp-vpa-b",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs: []api.VirtualParticipantAgent{
				{
					DeployableEntity: api.DeployableEntity{
						Entity: api.Entity{
							ID:      "vpa-3",
							Version: 1,
						},
						State:          api.DeploymentStateActive,
						StateTimestamp: time.Now(),
					},
					Type: "connector",
				},
			},
			Properties: map[string]any{},
			Error:      false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-vpa-3",
				Version: 1,
			},
			Identifier:          "corp-vpa-c",
			TenantID:            "tenant-2",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
	}

	for _, profile := range profiles {
		vpasJSON, err := json.Marshal(profile.VPAs)
		require.NoError(t, err)

		_, err = testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			[]byte("[]"),
			vpasJSON,
			profile.Error,
			profile.ErrorDetail,
			[]byte("{}"),
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("vpas.type", "connector")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		hasConnector := false
		for _, vpa := range profile.VPAs {
			if vpa.Type == "connector" {
				hasConnector = true
				break
			}
		}
		assert.True(t, hasConnector)
	}

	assert.Equal(t, 2, count)
}

// TestNewParticipantProfileStore_SearchByPropertiesPredicate tests filtering by properties
func TestNewParticipantProfileStore_SearchByPropertiesPredicate(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-prop-1",
				Version: 1,
			},
			Identifier:          "corp-prop-a",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties: map[string]any{
				"region": "us-east-1",
				"tier":   "premium",
				"status": "active",
			},
			Error: false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-prop-2",
				Version: 1,
			},
			Identifier:          "corp-prop-b",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties: map[string]any{
				"region": "eu-west-1",
				"tier":   "standard",
				"status": "active",
			},
			Error: false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-prop-3",
				Version: 1,
			},
			Identifier:          "corp-prop-c",
			TenantID:            "tenant-2",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties: map[string]any{
				"region": "ap-south-1",
				"tier":   "premium",
				"status": "inactive",
			},
			Error: false,
		},
	}

	for _, profile := range profiles {
		propsJSON, err := json.Marshal(profile.Properties)
		require.NoError(t, err)

		_, err = testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			[]byte("[]"),
			[]byte("[]"),
			profile.Error,
			profile.ErrorDetail,
			propsJSON,
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("properties.tier", "premium")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		tier, exists := profile.Properties["tier"]
		assert.True(t, exists)
		assert.Equal(t, "premium", tier)
	}

	assert.Equal(t, 2, count)
}

// TestNewParticipantProfileStore_SearchByErrorPredicate tests filtering by error field
func TestNewParticipantProfileStore_SearchByErrorPredicate(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-err-1",
				Version: 1,
			},
			Identifier:          "corp-err-a",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               true,
			ErrorDetail:         "deployment timeout",
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-err-2",
				Version: 1,
			},
			Identifier:          "corp-err-b",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
			ErrorDetail:         "",
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-err-3",
				Version: 1,
			},
			Identifier:          "corp-err-c",
			TenantID:            "tenant-2",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               true,
			ErrorDetail:         "network error",
		},
	}

	for _, profile := range profiles {
		_, err := testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			[]byte("[]"),
			[]byte("[]"),
			profile.Error,
			profile.ErrorDetail,
			[]byte("{}"),
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("error", true)

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		assert.True(t, profile.Error)
	}

	assert.Equal(t, 2, count)
}

// TestNewParticipantProfileStore_SearchByIdentifierPredicate tests filtering by identifier
func TestNewParticipantProfileStore_SearchByIdentifierPredicate(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-id-1",
				Version: 1,
			},
			Identifier:          "acme-corp-us",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-id-2",
				Version: 1,
			},
			Identifier:          "acme-corp-eu",
			TenantID:            "tenant-1",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-id-3",
				Version: 1,
			},
			Identifier:          "globex-corp",
			TenantID:            "tenant-2",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
	}

	for _, profile := range profiles {
		_, err := testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			[]byte("[]"),
			[]byte("[]"),
			profile.Error,
			profile.ErrorDetail,
			[]byte("{}"),
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Like("identifier", "acme%")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		assert.Contains(t, profile.Identifier, "acme")
	}

	assert.Equal(t, 2, count)
}

// TestNewParticipantProfileStore_SearchByTenantIDPredicate tests filtering by tenantID
func TestNewParticipantProfileStore_SearchByTenantIDPredicate(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-tenant-1",
				Version: 1,
			},
			Identifier:          "corp-tenant-a",
			TenantID:            "tenant-123",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-tenant-2",
				Version: 1,
			},
			Identifier:          "corp-tenant-b",
			TenantID:            "tenant-123",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-tenant-3",
				Version: 1,
			},
			Identifier:          "corp-tenant-c",
			TenantID:            "tenant-456",
			DataspaceProfileIDs: []string{},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties:          map[string]any{},
			Error:               false,
		},
	}

	for _, profile := range profiles {
		_, err := testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			[]byte("[]"),
			[]byte("[]"),
			profile.Error,
			profile.ErrorDetail,
			[]byte("{}"),
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.Eq("tenantId", "tenant-123")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		assert.Equal(t, "tenant-123", profile.TenantID)
	}

	assert.Equal(t, 2, count)
}

// TestNewParticipantProfileStore_SearchByCombinedPredicates tests filtering with multiple conditions
func TestNewParticipantProfileStore_SearchByCombinedPredicates(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-comb-1",
				Version: 1,
			},
			Identifier:          "corp-comb-a",
			TenantID:            "tenant-combined",
			DataspaceProfileIDs: []string{"dprofile-x"},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties: map[string]any{
				"status": "active",
			},
			Error: false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-comb-2",
				Version: 1,
			},
			Identifier:          "corp-comb-b",
			TenantID:            "tenant-combined",
			DataspaceProfileIDs: []string{"dprofile-x"},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties: map[string]any{
				"status": "inactive",
			},
			Error: false,
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-comb-3",
				Version: 1,
			},
			Identifier:          "corp-comb-c",
			TenantID:            "tenant-other",
			DataspaceProfileIDs: []string{"dprofile-x"},
			VPAs:                []api.VirtualParticipantAgent{},
			Properties: map[string]any{
				"status": "active",
			},
			Error: false,
		},
	}

	for _, profile := range profiles {
		dsProfileIdsJSON, err := json.Marshal(profile.DataspaceProfileIDs)
		require.NoError(t, err)
		propsJSON, err := json.Marshal(profile.Properties)
		require.NoError(t, err)

		_, err = testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			dsProfileIdsJSON,
			[]byte("[]"),
			profile.Error,
			profile.ErrorDetail,
			propsJSON,
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	predicate := query.And(
		query.Eq("tenantId", "tenant-combined"),
		query.Eq("properties.status", "active"),
	)

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		assert.Equal(t, "tenant-combined", profile.TenantID)
		status, _ := profile.Properties["status"]
		assert.Equal(t, "active", status)
	}

	assert.Equal(t, 1, count)
}

// TestNewParticipantProfileStore_SearchByParticipantRolesPredicate tests filtering by participantRoles
func TestNewParticipantProfileStore_SearchByParticipantRolesPredicate(t *testing.T) {
	setupParticipantProfileTable(t, testDB)
	defer cleanupParticipantProfileTestData(t, testDB)

	profiles := []*api.ParticipantProfile{
		{
			Entity: api.Entity{
				ID:      "pprofile-role-1",
				Version: 1,
			},
			Identifier:       "corp-role-a",
			TenantID:         "tenant-1",
			ParticipantRoles: map[string][]string{"dspace1": {"MembershipCredential", "RegistryRole"}},
			Properties:       map[string]any{},
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-role-2",
				Version: 1,
			},
			Identifier:       "corp-role-b",
			TenantID:         "tenant-1",
			ParticipantRoles: map[string][]string{"dspace1": {"MembershipCredential"}},
			Properties:       map[string]any{},
		},
		{
			Entity: api.Entity{
				ID:      "pprofile-role-3",
				Version: 1,
			},
			Identifier:       "corp-role-c",
			TenantID:         "tenant-2",
			ParticipantRoles: map[string][]string{"dspace2": {"MembershipCredential"}},
			Properties:       map[string]any{},
		},
	}

	for _, profile := range profiles {
		rolesJSON, err := json.Marshal(profile.ParticipantRoles)
		require.NoError(t, err)

		_, err = testDB.Exec(
			"INSERT INTO participant_profiles (id, version, identifier, tenant_id, participant_roles, dataspace_profile_ids, vpas, error, error_detail, properties) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
			profile.ID,
			profile.Version,
			profile.Identifier,
			profile.TenantID,
			rolesJSON,
			[]byte("[]"),
			[]byte("[]"),
			false,
			"",
			[]byte("{}"),
		)
		require.NoError(t, err)
	}

	estore := newParticipantProfileStore()
	ctx := context.Background()

	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, sqlstore.SQLTransactionKey, tx)

	// Search for profiles having "RegistryRole" in "dspace1"
	predicate := query.Eq("participantRoles.dspace1", "RegistryRole")

	count := 0
	for profile, err := range estore.FindByPredicatePaginated(txCtx, predicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		require.NotNil(t, profile)
		count++

		roles, exists := profile.ParticipantRoles["dspace1"]
		assert.True(t, exists)
		assert.Contains(t, roles, "RegistryRole")
	}

	assert.Equal(t, 1, count)
}

func setupParticipantProfileTable(t *testing.T, db *sql.DB) {
	err := createParticipantProfilesTable(db)
	require.NoError(t, err)
}

func cleanupParticipantProfileTestData(t *testing.T, db *sql.DB) {
	_, err := db.Exec("DROP TABLE IF EXISTS participant_profiles CASCADE")
	require.NoError(t, err)
}
