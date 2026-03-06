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

func newDataspaceProfileStore() store.EntityStore[*api.DataspaceProfile] {
	columnNames := []string{"id", "version", "dataspace_spec", "artifacts", "deployments", "properties"}
	builder := sqlstore.NewPostgresJSONBBuilder().WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
		"artifacts":      sqlstore.JSONBFieldTypeArrayOfScalars,
		"deployments":    sqlstore.JSONBFieldTypeArrayOfObjects,
		"dataspace_spec": sqlstore.JSONBFieldTypeScalar,
		"properties":     sqlstore.JSONBFieldTypeScalar,
	})

	estore := sqlstore.NewPostgresEntityStore[*api.DataspaceProfile](
		cfmDataspaceProfilesTable,
		columnNames,
		recordToDProfileEntity,
		dProfileEntityToRecord,
		builder,
	)

	return estore
}

func recordToDProfileEntity(_ *sql.Tx, record *sqlstore.DatabaseRecord) (*api.DataspaceProfile, error) {
	profile := &api.DataspaceProfile{}
	if id, ok := record.Values["id"].(string); ok {
		profile.ID = id
	} else {
		return nil, fmt.Errorf("invalid dataspace profile id reading record")
	}

	if version, ok := record.Values["version"].(int64); ok {
		profile.Version = version
	} else {
		return nil, fmt.Errorf("invalid dataspace profile version reading record")
	}

	if bytes, ok := record.Values["dataspace_spec"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.DataspaceSpec); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["artifacts"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.Artifacts); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["deployments"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.Deployments); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["properties"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.Properties); err != nil {
			return nil, err
		}
	}
	return profile, nil

}

func dProfileEntityToRecord(profile *api.DataspaceProfile) (*sqlstore.DatabaseRecord, error) {
	record := &sqlstore.DatabaseRecord{
		Values: make(map[string]any),
	}

	record.Values["id"] = profile.ID
	record.Values["version"] = profile.Version

	bytes, err := json.Marshal(profile.DataspaceSpec)
	if err != nil {
		return record, err
	}
	record.Values["dataspace_spec"] = bytes

	if profile.Artifacts != nil {
		bytes, err := json.Marshal(profile.Artifacts)
		if err != nil {
			return record, err
		}
		record.Values["artifacts"] = bytes
	}

	if profile.Deployments != nil {
		bytes, err := json.Marshal(profile.Deployments)
		if err != nil {
			return record, err
		}
		record.Values["deployments"] = bytes
	}

	if profile.Properties != nil {
		bytes, err := json.Marshal(profile.Properties)
		if err != nil {
			return record, err
		}
		record.Values["properties"] = bytes
	}

	return record, nil
}
