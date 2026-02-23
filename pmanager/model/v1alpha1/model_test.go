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

package v1alpha1

import (
	"testing"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityDefinitionTypeValidation(t *testing.T) {

	tests := []struct {
		name    string
		actDef  ActivityDefinitionDto
		wantErr bool
	}{
		{
			name: "valid type with alphanumeric and allowed chars",
			actDef: ActivityDefinitionDto{
				Type: "my-activity_v1.0",
			},
			wantErr: false,
		},
		{
			name: "valid type with only alphanumeric",
			actDef: ActivityDefinitionDto{
				Type: "SimpleActivity",
			},
			wantErr: false,
		},
		{
			name: "valid type with dots and underscores",
			actDef: ActivityDefinitionDto{
				Type: "com.example.activity_name",
			},
			wantErr: false,
		},
		{
			name: "valid type with hyphens",
			actDef: ActivityDefinitionDto{
				Type: "my-activity-type",
			},
			wantErr: false,
		},
		{
			name: "valid type with numbers",
			actDef: ActivityDefinitionDto{
				Type: "activity123",
			},
			wantErr: false,
		},
		{
			name: "invalid type with space",
			actDef: ActivityDefinitionDto{
				Type: "invalid activity",
			},
			wantErr: true,
		},
		{
			name: "invalid type with special chars @",
			actDef: ActivityDefinitionDto{
				Type: "invalid@activity",
			},
			wantErr: true,
		},
		{
			name: "invalid type with special chars #",
			actDef: ActivityDefinitionDto{
				Type: "invalid#activity",
			},
			wantErr: true,
		},
		{
			name: "invalid type with special chars $",
			actDef: ActivityDefinitionDto{
				Type: "invalid$activity",
			},
			wantErr: true,
		},
		{
			name: "invalid type with special chars %",
			actDef: ActivityDefinitionDto{
				Type: "invalid%activity",
			},
			wantErr: true,
		},
		{
			name: "invalid type with special chars &",
			actDef: ActivityDefinitionDto{
				Type: "invalid&activity",
			},
			wantErr: true,
		},
		{
			name: "invalid type empty string",
			actDef: ActivityDefinitionDto{
				Type: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := model.Validator.Var(tt.actDef.Type, "required,modeltype")
			if tt.wantErr {
				require.Error(t, err, "expected validation error")
			} else {
				require.NoError(t, err, "expected no validation error")
			}
		})
	}
}

func TestActivityTypeValidation(t *testing.T) {

	tests := []struct {
		name     string
		activity ActivityDto
		wantErr  bool
	}{
		{
			name: "valid activity type",
			activity: ActivityDto{
				ID:   "act-1",
				Type: "process-data",
			},
			wantErr: false,
		},
		{
			name: "valid activity type with dots",
			activity: ActivityDto{
				ID:   "act-2",
				Type: "com.example.ProcessActivity",
			},
			wantErr: false,
		},
		{
			name: "valid activity type with underscores",
			activity: ActivityDto{
				ID:   "act-3",
				Type: "process_data_activity",
			},
			wantErr: false,
		},
		{
			name: "valid activity type with numbers",
			activity: ActivityDto{
				ID:   "act-4",
				Type: "activity2process3",
			},
			wantErr: false,
		},
		{
			name: "valid activity type with version",
			activity: ActivityDto{
				ID:   "act-5",
				Type: "activity.v1.0",
			},
			wantErr: false,
		},
		{
			name: "invalid activity type with space",
			activity: ActivityDto{
				ID:   "act-6",
				Type: "invalid type",
			},
			wantErr: true,
		},
		{
			name: "invalid activity type with @",
			activity: ActivityDto{
				ID:   "act-7",
				Type: "invalid@type",
			},
			wantErr: true,
		},
		{
			name: "invalid activity type with #",
			activity: ActivityDto{
				ID:   "act-8",
				Type: "invalid#type",
			},
			wantErr: true,
		},
		{
			name: "invalid activity type with $",
			activity: ActivityDto{
				ID:   "act-9",
				Type: "invalid$type",
			},
			wantErr: true,
		},
		{
			name: "invalid activity type with %",
			activity: ActivityDto{
				ID:   "act-10",
				Type: "invalid%type",
			},
			wantErr: true,
		},
		{
			name: "invalid activity type with &",
			activity: ActivityDto{
				ID:   "act-11",
				Type: "invalid&type",
			},
			wantErr: true,
		},
		{
			name: "invalid activity type empty",
			activity: ActivityDto{
				ID:   "act-12",
				Type: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := model.Validator.Var(tt.activity.Type, "required,modeltype")
			if tt.wantErr {
				assert.Error(t, err, "expected validation error for type: %s", tt.activity.Type)
			} else {
				assert.NoError(t, err, "expected no validation error for type: %s", tt.activity.Type)
			}
		})
	}
}

//func TestOrchestrationDefinitionTypeValidation(t *testing.T) {
//	tests := []struct {
//		name    string
//		orchDef OrchestrationDefinitionDto
//		wantErr bool
//	}{
//		{
//			name: "valid orchestration type",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "sequential-orchestration",
//			},
//			wantErr: false,
//		},
//		{
//			name: "valid orchestration type with version",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "orchestration.v1.0",
//			},
//			wantErr: false,
//		},
//		{
//			name: "valid orchestration type with namespace",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "com.company.orchestration_type",
//			},
//			wantErr: false,
//		},
//		{
//			name: "valid orchestration type with numbers",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "orch123-type",
//			},
//			wantErr: false,
//		},
//		{
//			name: "valid orchestration type simple",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "Orchestration",
//			},
//			wantErr: false,
//		},
//		{
//			name: "invalid orchestration type with space",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "invalid orchestration",
//			},
//			wantErr: true,
//		},
//		{
//			name: "invalid orchestration type with @",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "invalid@orchestration",
//			},
//			wantErr: true,
//		},
//		{
//			name: "invalid orchestration type with #",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "invalid#orchestration",
//			},
//			wantErr: true,
//		},
//		{
//			name: "invalid orchestration type with $",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "invalid$orchestration",
//			},
//			wantErr: true,
//		},
//		{
//			name: "invalid orchestration type with %",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "invalid%orchestration",
//			},
//			wantErr: true,
//		},
//		{
//			name: "invalid orchestration type with &",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "invalid&orchestration",
//			},
//			wantErr: true,
//		},
//		{
//			name: "invalid orchestration type empty",
//			orchDef: OrchestrationDefinitionDto{
//				Type: "",
//			},
//			wantErr: true,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			err := model.Validator.Var(tt.orchDef.Type, "required,modeltype")
//			if tt.wantErr {
//				assert.Error(t, err, "expected validation error for type: %s", tt.orchDef.Type)
//			} else {
//				assert.NoError(t, err, "expected no validation error for type: %s", tt.orchDef.Type)
//			}
//		})
//	}
//}
