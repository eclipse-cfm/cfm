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
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/tmanager/api"
)

func newTenantStore() store.EntityStore[*api.Tenant] {
	columnNames := []string{"id", "version", "properties"}
	builder := sqlstore.NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
		"properties": sqlstore.JSONBFieldTypeScalar,
	})

	estore := sqlstore.NewPostgresEntityStore[*api.Tenant](
		cfmTenantsTable,
		columnNames,
		recordToTenantEntity,
		tenantEntityToRecord,
		builder,
	)

	return estore
}

func recordToTenantEntity(_ *sql.Tx, record *sqlstore.DatabaseRecord) (*api.Tenant, error) {
	profile := &api.Tenant{}
	if id, ok := record.Values["id"].(string); ok {
		profile.ID = id
	} else {
		return nil, fmt.Errorf("invalid tenant id reading record")
	}

	if version, ok := record.Values["version"].(int64); ok {
		profile.Version = version
	} else {
		return nil, fmt.Errorf("invalid tenant version reading record")
	}

	if bytes, ok := record.Values["properties"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.Properties); err != nil {
			return nil, err
		}
	}
	return profile, nil

}

func tenantEntityToRecord(profile *api.Tenant) (*sqlstore.DatabaseRecord, error) {
	record := &sqlstore.DatabaseRecord{
		Values: make(map[string]any),
	}

	record.Values["id"] = profile.ID
	record.Values["version"] = profile.Version

	if profile.Properties != nil {
		bytes, err := json.Marshal(profile.Properties)
		if err != nil {
			return record, err
		}
		record.Values["properties"] = bytes
	}

	return record, nil
}
