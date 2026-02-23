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

package e2efixtures

import (
	"fmt"
	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	pv1alpha1 "github.com/metaform/connector-fabric-manager/pmanager/model/v1alpha1"
	tv1alpha1 "github.com/metaform/connector-fabric-manager/tmanager/model/v1alpha1"
)

const (
	MembershipCredential = "MembershipCredential"
	GovernanceCredential = "GovernanceCredential"
	OEMRole              = "oem"
	IssuerDID            = "did:web:issuer.com"
)

func CreateTestActivityDefinition(apiClient *ApiClient) error {
	requestBody := api.ActivityDefinition{
		Type:        "test-activity",
		Description: "Performs a test activity",
	}

	return apiClient.PostToPManager("activity-definitions", requestBody)
}

func CreateTestOrchestrationDefinitions(apiClient *ApiClient) error {
	requestBody := pv1alpha1.OrchestrationTemplate{
		Activities: map[string][]pv1alpha1.ActivityDto{
			model.VPADeployType.String(): {{
				ID:   "activity1",
				Type: "test-activity",
			}},
			model.VPADisposeType.String(): {{
				ID:   "activity2",
				Type: "test-activity",
			}},
		},
	}

	return apiClient.PostToPManager("orchestration-definitions", requestBody)
}

func CreateCell(apiClient *ApiClient) (*tv1alpha1.Cell, error) {
	requestBody := tv1alpha1.NewCell{
		State:          "active",
		StateTimestamp: time.Time{}.UTC(),
		Properties:     make(map[string]any),
	}
	var cell tv1alpha1.Cell
	err := apiClient.PostToTManagerWithResponse("cells", requestBody, &cell)
	if err != nil {
		return nil, err
	}
	return &cell, nil
}

func CreateTenant(apiClient *ApiClient, properties map[string]any) (*tv1alpha1.Tenant, error) {
	requestBody := tv1alpha1.NewTenant{Properties: properties}
	var tenant tv1alpha1.Tenant
	err := apiClient.PostToTManagerWithResponse("tenants", requestBody, &tenant)
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}

func CreateDataspaceProfile(apiClient *ApiClient) (*tv1alpha1.DataspaceProfile, error) {
	requestBody := tv1alpha1.NewDataspaceProfile{
		Artifacts:  make([]string, 0),
		Properties: make(map[string]any),
		DataspaceSpec: tv1alpha1.DataspaceSpec{
			ProtocolStack: []string{"dsp-2025-1", "dcp-2025-1"},
			CredentialSpecs: []tv1alpha1.CredentialSpec{
				{
					Type:   MembershipCredential,
					Issuer: IssuerDID,
					Format: "VC1_0_JWT",
				},
				{
					Type:            GovernanceCredential,
					Issuer:          IssuerDID,
					Format:          "VC1_0_JWT",
					ParticipantRole: OEMRole,
				}},
		},
	}
	var profile tv1alpha1.DataspaceProfile
	err := apiClient.PostToTManagerWithResponse("dataspace-profiles", requestBody, &profile)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func DeployDataspaceProfile(deployment tv1alpha1.NewDataspaceProfileDeployment, apiClient *ApiClient) error {
	err := apiClient.PostToTManager(fmt.Sprintf("dataspace-profiles/%s/deployments", deployment.ProfileID), deployment)
	if err != nil {
		return err
	}
	return nil
}
