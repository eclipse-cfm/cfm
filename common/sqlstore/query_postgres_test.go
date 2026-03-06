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
	"encoding/json"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/query"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModel represents the domain model structure for integration testing
type TestModel struct {
	ID                string     `json:"id"`
	Identifier        string     `json:"identifier"`
	TenantID          string     `json:"tenantId"`
	VPAs              []VPA      `json:"vpas"`
	Properties        Properties `json:"properties"`
	CreatedAt         time.Time  `json:"createdAt"`
	DataspaceProfiles []Profile  `json:"dataspaceProfiles"`
}

// VPA represents a Virtual Participant Agent with nested Cell info
type VPA struct {
	ID    string     `json:"id"`
	Type  string     `json:"type"`
	Cell  Cell       `json:"cell"`
	State string     `json:"state"`
	Props Properties `json:"properties"`
}

// Cell represents a deployment zone
type Cell struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Name  string `json:"name"`
}

// Profile represents a dataspace profile
type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Properties is a map of extensible attributes
type Properties map[string]any

// setupTestTable creates the test table with JSONB columns
func setupTestTable(t *testing.T) {
	_, err := testDB.Exec(`
		DROP TABLE IF EXISTS participant_profiles CASCADE;
		CREATE TABLE participant_profiles (
			id TEXT PRIMARY KEY,
			identifier TEXT NOT NULL,
			tenantid TEXT NOT NULL,
			vpas JSONB NOT NULL DEFAULT '[]',
			properties JSONB,
			dataspace_profiles JSONB,
			participant_roles JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			version INT DEFAULT 1
		);
	`)
	require.NoError(t, err)
}

// insertTestData inserts test participant profile data
func insertTestData(t *testing.T, model TestModel) {
	vpasJSON, err := json.Marshal(model.VPAs)
	require.NoError(t, err)

	propsJSON, err := json.Marshal(model.Properties)
	require.NoError(t, err)

	profilesJSON, err := json.Marshal(model.DataspaceProfiles)
	require.NoError(t, err)

	participantRolesJSON, err := json.Marshal(map[string][]string{"dspace1": {"MembershipCredential", "RegistryRole"}})
	require.NoError(t, err)

	_, err = testDB.Exec(`
		INSERT INTO participant_profiles 
		(id, identifier, tenantid, vpas, properties, dataspace_profiles, participant_roles, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, model.ID, model.Identifier, model.TenantID, vpasJSON, propsJSON, profilesJSON, participantRolesJSON, model.CreatedAt)
	require.NoError(t, err)
}

// TestPostgresJSONB_QueryVPAsSimpleEquality tests simple JSONB equality on VPA fields
func TestPostgresJSONB_QueryVPAsSimpleEquality(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test data
	testData := TestModel{
		ID:         "pp1",
		Identifier: "org-a",
		TenantID:   "tenant1",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				Cell:  Cell{ID: "cell1", State: "active", Name: "Cell1"},
				State: "active",
				Props: Properties{"env": "prod"},
			},
			{
				ID:    "vpa2",
				Type:  "credential-service",
				Cell:  Cell{ID: "cell2", State: "active", Name: "Cell2"},
				State: "pending",
				Props: Properties{"env": "staging"},
			},
		},
		Properties: Properties{"region": "us-east", "owner": "admin"},
		CreatedAt:  time.Now(),
	}
	insertTestData(t, testData)

	// Verify data was inserted
	var checkCount int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles").Scan(&checkCount)
	require.NoError(t, err)

	// Verify JSONB data structure
	var vpasRaw string
	err = testDB.QueryRow("SELECT vpas FROM participant_profiles WHERE id = $1", "pp1").Scan(&vpasRaw)
	require.NoError(t, err)

	// Build query for VPA Type = "connector"
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.Eq("vpas.type", "connector")
	sqlStatement, args := builder.BuildSQL(predicate)

	// Execute query
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStatement, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryNestedCellID tests nested JSONB queries (VPAs.CellID.ID)
func TestPostgresJSONB_QueryNestedCellID(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	// Insert test data with specific cell IDs
	testData := TestModel{
		ID:         "pp2",
		Identifier: "org-b",
		TenantID:   "tenant1",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				Cell:  Cell{ID: "cell-prod", State: "active"},
				State: "active",
			},
			{
				ID:    "vpa2",
				Type:  "connector",
				Cell:  Cell{ID: "cell-staging", State: "active"},
				State: "active",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query for VPAs with CellID.ID = "cell-prod"
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.Eq("vpas.cell.id", "cell-prod")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryCompoundAND tests compound AND queries
func TestPostgresJSONB_QueryCompoundAND(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp3",
		Identifier: "org-c",
		TenantID:   "tenant1",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				Cell:  Cell{ID: "cell1", State: "active"},
				State: "active",
			},
			{
				ID:    "vpa2",
				Type:  "credential-service",
				Cell:  Cell{ID: "cell1", State: "active"},
				State: "pending",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query: Type = "connector" AND CellID.ID = "cell1"
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	compound := query.And(
		query.Eq("vpas.type", "connector"),
		query.Eq("vpas.cell.id", "cell1"),
	)
	sqlStr, args := builder.BuildSQL(compound)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryCompoundOR tests compound OR queries
func TestPostgresJSONB_QueryCompoundOR(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp4",
		Identifier: "org-d",
		TenantID:   "tenant1",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				Cell:  Cell{ID: "cell1"},
				State: "active",
			},
			{
				ID:    "vpa2",
				Type:  "credential-service",
				Cell:  Cell{ID: "cell2"},
				State: "pending",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query: Type = "connector" OR Type = "credential-service"
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	compound := query.Or(
		query.Eq("vpas.type", "connector"),
		query.Eq("vpas.type", "credential-service"),
	)
	sqlStr, args := builder.BuildSQL(compound)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryInOperator tests IN operator for multiple values
func TestPostgresJSONB_QueryInOperator(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp5",
		Identifier: "org-e",
		TenantID:   "tenant2",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				Cell:  Cell{ID: "cell1"},
				State: "active",
			},
			{
				ID:    "vpa2",
				Type:  "connector",
				Cell:  Cell{ID: "cell2"},
				State: "disposed",
			},
			{
				ID:    "vpa3",
				Type:  "connector",
				Cell:  Cell{ID: "cell3"},
				State: "pending",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query: Type IN ("connector", "credential-service")
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.In("vpas.type", "connector", "credential-service")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryNotInOperator tests NOT IN operator
func TestPostgresJSONB_QueryNotInOperator(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	// Insert first record with VPAs that should NOT be excluded
	testData1 := TestModel{
		ID:         "pp6a",
		Identifier: "org-f-1",
		TenantID:   "tenant2",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				Cell:  Cell{ID: "cell1"},
				State: "active",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData1)

	// Insert second record with VPAs that should NOT be excluded
	testData2 := TestModel{
		ID:         "pp6b",
		Identifier: "org-f-2",
		TenantID:   "tenant2",
		VPAs: []VPA{
			{
				ID:    "vpa2",
				Type:  "connector",
				Cell:  Cell{ID: "cell2"},
				State: "pending",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData2)

	// Insert third record with VPAs that SHOULD be excluded
	testData3 := TestModel{
		ID:         "pp6c",
		Identifier: "org-f-3",
		TenantID:   "tenant2",
		VPAs: []VPA{
			{
				ID:    "vpa3",
				Type:  "connector",
				Cell:  Cell{ID: "cell3"},
				State: "disposed",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData3)

	// Query: State NOT IN ("disposed", "deleted")
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.NotIn("vpas.state", "disposed", "deleted")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// TestPostgresJSONB_QueryNotEqual tests not-equal operator
func TestPostgresJSONB_QueryNotEqual(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp7",
		Identifier: "org-g",
		TenantID:   "tenant3",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				State: "active",
			},
			{
				ID:    "vpa2",
				Type:  "connector",
				State: "pending",
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query: State != "disposed"
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.Neq("vpas.state", "disposed")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryIsNull tests IS NULL operator
func TestPostgresJSONB_QueryIsNull(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	// Insert data with null Properties
	_, err := testDB.Exec(`
		INSERT INTO participant_profiles 
		(id, identifier, tenantid, vpas, properties, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "pp8", "org-h", "tenant3", "[]", nil, time.Now())
	require.NoError(t, err)

	// Insert data with non-null Properties
	_, err = testDB.Exec(`
		INSERT INTO participant_profiles 
		(id, identifier, tenantid, vpas, properties, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "pp9", "org-i", "tenant3", "[]", `{"key":"value"}`, time.Now())
	require.NoError(t, err)

	// Query: Properties IS NULL
	builder := NewPostgresJSONBBuilder().WithJSONBFields("Properties")
	predicate := query.IsNull("Properties")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryIsNotNull tests IS NOT NULL operator
func TestPostgresJSONB_QueryIsNotNull(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	_, err := testDB.Exec(`
		INSERT INTO participant_profiles 
		(id, identifier, tenantid, vpas, properties, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "pp10", "org-j", "tenant4", "[]", nil, time.Now())
	require.NoError(t, err)

	_, err = testDB.Exec(`
		INSERT INTO participant_profiles 
		(id, identifier, tenantid, vpas, properties, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, "pp11", "org-k", "tenant4", "[]", `{"key":"value"}`, time.Now())
	require.NoError(t, err)

	// Query: Properties IS NOT NULL
	builder := NewPostgresJSONBBuilder().WithJSONBFields("Properties")
	predicate := query.IsNotNull("Properties")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryContains tests JSONB @> (contains) operator
func TestPostgresJSONB_QueryContains(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp12",
		Identifier: "org-l",
		TenantID:   "tenant5",
		VPAs:       []VPA{},
		Properties: Properties{"region": "us-east", "env": "prod", "owner": "admin"},
		CreatedAt:  time.Now(),
	}
	insertTestData(t, testData)

	// Query: Properties contains {"env":"prod"}
	builder := NewPostgresJSONBBuilder().WithJSONBFields("Properties")
	predicate := query.NewComparison("Properties", query.OpContains, `{"env":"prod"}`)
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryMultipleRecordsWithDifferentCells tests querying across multiple records
func TestPostgresJSONB_QueryMultipleRecordsWithDifferentCells(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	// Insert multiple records with different VPA configurations
	records := []TestModel{
		{
			ID:         "pp13",
			Identifier: "org-m",
			TenantID:   "tenant6",
			VPAs: []VPA{
				{ID: "vpa1", Type: "connector", Cell: Cell{ID: "cell-a"}},
			},
			CreatedAt: time.Now(),
		},
		{
			ID:         "pp14",
			Identifier: "org-n",
			TenantID:   "tenant6",
			VPAs: []VPA{
				{ID: "vpa2", Type: "connector", Cell: Cell{ID: "cell-b"}},
			},
			CreatedAt: time.Now(),
		},
		{
			ID:         "pp15",
			Identifier: "org-o",
			TenantID:   "tenant6",
			VPAs: []VPA{
				{ID: "vpa3", Type: "credential-service", Cell: Cell{ID: "cell-a"}},
			},
			CreatedAt: time.Now(),
		},
	}

	for _, record := range records {
		insertTestData(t, record)
	}

	// Query: CellID.ID = "cell-a"
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.Eq("vpas.cell.id", "cell-a")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// TestPostgresJSONB_QueryCombinedANDOR tests complex compound predicates (AND/OR combinations)
func TestPostgresJSONB_QueryCombinedANDOR(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp16",
		Identifier: "org-p",
		TenantID:   "tenant7",
		VPAs: []VPA{
			{
				ID:    "vpa1",
				Type:  "connector",
				State: "active",
				Cell:  Cell{ID: "cell1"},
			},
			{
				ID:    "vpa2",
				Type:  "credential-service",
				State: "pending",
				Cell:  Cell{ID: "cell2"},
			},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query: (Type = "connector" AND State = "active") OR (Type = "credential-service")
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	compound := query.Or(
		query.And(
			query.Eq("vpas.type", "connector"),
			query.Eq("vpas.state", "active"),
		),
		query.Eq("VPAs.Type", "credential-service"),
	)
	sqlStr, args := builder.BuildSQL(compound)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryNonJSONBFieldsWithJSONBFields tests mixing JSONB and regular fields
func TestPostgresJSONB_QueryNonJSONBFieldsWithJSONBFields(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp17",
		Identifier: "org-q",
		TenantID:   "tenant8",
		VPAs: []VPA{
			{ID: "vpa1", Type: "connector", State: "active"},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query: tenantid = "tenant8" AND VPAs.Type = "connector"
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	compound := query.And(
		query.Eq("tenantid", "tenant8"),
		query.Eq("vpas.type", "connector"),
	)
	sqlStr, args := builder.BuildSQL(compound)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryNonJSONBFieldsWithJSONBFields tests field name mappings
func TestPostgresJSONB_QueryFieldMapping(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp17",
		Identifier: "org-q",
		TenantID:   "tenant8",
		VPAs: []VPA{
			{ID: "vpa1", Type: "connector", State: "active"},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query: tenant_id = "tenant8" AND VPAs.Type = "connector"
	// the tenant_id field is mapped to the tenantId column by the JSONB type is not mnapped
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs").WithFieldMappings(map[string]string{"tenant_id": "tenantId", "Type": "no_rename_jsonb"})
	compound := query.And(
		query.Eq("tenant_id", "tenant8"),
		query.Eq("vpas.type", "connector"),
	)
	sqlStr, args := builder.BuildSQL(compound)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryEmptyVPAsArray tests records with empty VPA arrays
func TestPostgresJSONB_QueryEmptyVPAsArray(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp18",
		Identifier: "org-r",
		TenantID:   "tenant9",
		VPAs:       []VPA{},
		CreatedAt:  time.Now(),
	}
	insertTestData(t, testData)

	// Query: VPAs.Type = "connector" (should find nothing)
	builder := NewPostgresJSONBBuilder().WithJSONBFields("vpas")
	predicate := query.Eq("vpas.type", "connector")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestPostgresJSONB_QueryCaseInsensitiveField tests case-insensitive JSONB field configuration
func TestPostgresJSONB_QueryCaseInsensitiveField(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp19",
		Identifier: "org-s",
		TenantID:   "tenant10",
		VPAs: []VPA{
			{ID: "vpa1", Type: "connector", State: "active"},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Register field as uppercase, query with lowercase
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.Eq("vpas.type", "connector")
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QuerySelectSpecificColumns tests retrieving actual data from JSONB queries
func TestPostgresJSONB_QuerySelectSpecificColumns(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp20",
		Identifier: "org-t",
		TenantID:   "tenant11",
		VPAs: []VPA{
			{ID: "vpa1", Type: "connector", State: "active"},
		},
		CreatedAt: time.Now(),
	}
	insertTestData(t, testData)

	// Query and retrieve specific fields
	builder := NewPostgresJSONBBuilder().WithJSONBFields("VPAs")
	predicate := query.Eq("vpas.type", "connector")
	sqlStr, args := builder.BuildSQL(predicate)

	var id, identifier string
	err := testDB.QueryRow("SELECT id, identifier FROM participant_profiles WHERE "+sqlStr, args...).Scan(&id, &identifier)
	require.NoError(t, err)
	assert.Equal(t, "pp20", id)
	assert.Equal(t, "org-t", identifier)
}

// TestPostgresJSONB_QueryGreaterThanComparison tests numeric comparisons in JSONB
func TestPostgresJSONB_QueryGreaterThanComparison(t *testing.T) {
	// Create custom table with numeric JSONB fields
	_, err := testDB.Exec(`
		DROP TABLE IF EXISTS test_numeric CASCADE;
		CREATE TABLE test_numeric (
			id TEXT PRIMARY KEY,
			data JSONB NOT NULL
		);
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = testDB.Exec("DROP TABLE IF EXISTS test_numeric CASCADE;")
	}()

	// Insert test data with numeric properties
	_, err = testDB.Exec(`
		INSERT INTO test_numeric (id, data)
		VALUES 
			('r1', '[{"priority":5}]'),
			('r2', '[{"priority":10}]'),
			('r3', '[{"priority":3}]')
	`)
	require.NoError(t, err)

	// Query: priority > 5
	builder := NewPostgresJSONBBuilder().WithJSONBFields("data")
	predicate := query.Gt("data.priority", 5)
	sqlStr, args := builder.BuildSQL(predicate)

	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM test_numeric WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestPostgresJSONB_QueryMapOfArraysWithMapping tests querying a map of arrays using JSONBFieldTypeMapOfArrays
func TestPostgresJSONB_QueryMapOfArraysWithMapping(t *testing.T) {
	setupTestTable(t)
	defer CleanupTestData(t, testDB)

	testData := TestModel{
		ID:         "pp-roles-1",
		Identifier: "org-roles",
		TenantID:   "tenant-roles",
		CreatedAt:  time.Now(),
	}
	insertTestData(t, testData)

	// Configure builder with field mapping from testValues to participant_roles
	// and register testValues as JSONBFieldTypeMapOfArrays
	builder := NewPostgresJSONBBuilder().
		WithFieldMappings(map[string]string{"testValues": "participant_roles"}).
		WithJSONBFieldTypes(map[string]JSONBFieldType{
			"testValues": JSONBFieldTypeMapOfArrays,
		})

	// Query: testValues.dspace1 = "RegistryRole"
	// Should generate: participant_roles->'dspace1' @> jsonb_build_array($1::text)
	predicate := query.Eq("testValues.dspace1", "RegistryRole")
	sqlStr, args := builder.BuildSQL(predicate)

	assert.Contains(t, sqlStr, "participant_roles")
	assert.Contains(t, sqlStr, "@>")

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM participant_profiles WHERE "+sqlStr, args...).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
