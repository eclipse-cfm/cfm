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
	"github.com/eclipse-cfm/cfm/pmanager/api"
	pv1alpha1 "github.com/eclipse-cfm/cfm/pmanager/model/v1alpha1"
)

const (
	// DeployType and DisposeType are the orchestration types of the test orchestration definitions
	DeployType  = "cfm.orchestration.test.deploy"
	DisposeType = "cfm.orchestration.test.dispose"
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
			DeployType: {{
				ID:   "activity1",
				Type: "test-activity",
			}},
			DisposeType: {{
				ID:   "activity2",
				Type: "test-activity",
			}},
		},
		// a failed deployment is rolled back by the dispose orchestration
		Compensation: map[string]string{
			DeployType: DisposeType,
		},
	}

	return apiClient.PostToPManager("orchestration-definitions", requestBody)
}
