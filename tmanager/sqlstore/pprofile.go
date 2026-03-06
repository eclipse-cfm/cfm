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

func newParticipantProfileStore() store.EntityStore[*api.ParticipantProfile] {
	columnNames := []string{"id",
		"version",
		"identifier",
		"tenant_id",
		"dataspace_profile_ids",
		"participant_roles",
		"vpas",
		"error",
		"error_detail",
		"properties"}

	builder := sqlstore.NewPostgresJSONBBuilder().
		WithFieldMappings(map[string]string{
			"tenantId":            "tenant_id",
			"dataspaceProfileIds": "dataspace_profile_ids",
			"participantRoles":    "participant_roles",
			"errorDetail":         "error_detail",
		}).
		WithJSONBFieldTypes(map[string]sqlstore.JSONBFieldType{
			"dataspaceProfileIds": sqlstore.JSONBFieldTypeArrayOfScalars,
			"vpas":                sqlstore.JSONBFieldTypeArrayOfObjects,
			"participantRoles":    sqlstore.JSONBFieldTypeMapOfArrays,
			"properties":          sqlstore.JSONBFieldTypeScalar,
		})

	estore := sqlstore.NewPostgresEntityStore[*api.ParticipantProfile](
		cfmParticipantProfilesTable,
		columnNames,
		recordToPProfileEntity,
		pProfileEntityToRecord,
		builder,
	)

	return estore
}

func recordToPProfileEntity(_ *sql.Tx, record *sqlstore.DatabaseRecord) (*api.ParticipantProfile, error) {
	profile := &api.ParticipantProfile{}

	if id, ok := record.Values["id"].(string); ok {
		profile.ID = id
	} else {
		return nil, fmt.Errorf("invalid participant profile ID reading record")
	}

	if version, ok := record.Values["version"].(int64); ok {
		profile.Version = version
	} else {
		return nil, fmt.Errorf("invalid participant profile version reading record")
	}

	if tenantId, ok := record.Values["tenant_id"].(string); ok {
		profile.TenantID = tenantId
	} else {
		return nil, fmt.Errorf("invalid participant profile tenant ID reading record")
	}

	if identifier, ok := record.Values["identifier"].(string); ok {
		profile.Identifier = identifier
	} else {
		return nil, fmt.Errorf("invalid participant profile identifier reading record")
	}

	if err, ok := record.Values["error"].(bool); ok {
		profile.Error = err
	} else {
		return nil, fmt.Errorf("invalid participant profile error value reading record")
	}

	if errorDetail, ok := record.Values["error_detail"].(string); ok {
		profile.ErrorDetail = errorDetail
	} else {
		return nil, fmt.Errorf("invalid participant profile error detail reading record")
	}

	if bytes, ok := record.Values["properties"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.Properties); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["participant_roles"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.ParticipantRoles); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["vpas"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.VPAs); err != nil {
			return nil, err
		}
	}

	if bytes, ok := record.Values["dataspace_profile_ids"].([]byte); ok && bytes != nil {
		if err := json.Unmarshal(bytes, &profile.DataspaceProfileIDs); err != nil {
			return nil, err
		}
	}

	return profile, nil
}

func pProfileEntityToRecord(profile *api.ParticipantProfile) (*sqlstore.DatabaseRecord, error) {
	record := &sqlstore.DatabaseRecord{
		Values: make(map[string]any),
	}

	record.Values["id"] = profile.ID
	record.Values["version"] = profile.Version
	record.Values["tenant_id"] = profile.TenantID
	record.Values["identifier"] = profile.Identifier
	record.Values["error"] = profile.Error
	record.Values["error_detail"] = profile.ErrorDetail

	if profile.DataspaceProfileIDs != nil {
		bytes, err := json.Marshal(profile.DataspaceProfileIDs)
		if err != nil {
			return record, err
		}
		record.Values["dataspace_profile_ids"] = bytes
	}

	if profile.VPAs != nil {
		bytes, err := json.Marshal(profile.VPAs)
		if err != nil {
			return record, err
		}
		record.Values["vpas"] = bytes
	}

	if profile.Properties != nil {
		bytes, err := json.Marshal(profile.Properties)
		if err != nil {
			return record, err
		}
		record.Values["properties"] = bytes
	}

	if profile.ParticipantRoles != nil {
		bytes, err := json.Marshal(profile.ParticipantRoles)
		if err != nil {
			return record, err
		}
		record.Values["participant_roles"] = bytes
	}

	return record, nil
}
