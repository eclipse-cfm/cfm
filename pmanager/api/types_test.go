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

package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestOrchestration_CanProceedToNextStep(t *testing.T) {
	tests := []struct {
		name          string
		orchestration *Orchestration
		activityID    string
		want          bool
		wantErr       bool
	}{
		{
			name: "single step orchestration",
			orchestration: &Orchestration{
				Steps: []OrchestrationStep{
					{
						Activities: []Activity{
							{ID: "act1", Type: "test"},
						},
					},
				},
			},
			activityID: "act1",
			want:       true,
			wantErr:    false,
		},
		{
			name: "multiple steps - activity in first step",
			orchestration: &Orchestration{
				Steps: []OrchestrationStep{
					{
						Activities: []Activity{
							{ID: "act1", Type: "test"},
							{ID: "act2", Type: "test"},
						},
					},
					{
						Activities: []Activity{
							{ID: "act3", Type: "test"},
						},
					},
				},
			},
			activityID: "act1",
			want:       false, // Cannot proceed while other activities in step are pending
			wantErr:    false,
		},
		{
			name: "multiple steps - last activity in step",
			orchestration: &Orchestration{
				Steps: []OrchestrationStep{
					{
						Activities: []Activity{
							{ID: "act1", Type: "test"},
							{ID: "act2", Type: "test"},
						},
					},
					{
						Activities: []Activity{
							{ID: "act3", Type: "test"},
						},
					},
				},
				Completed: map[string]struct{}{"act1": {}},
			},
			activityID: "act2", // Last activity in first step
			want:       true,   // Should be true - can proceed to next step when this is the last activity in the step that needs to be completed
			wantErr:    false,
		},
		{
			name: "step with all activities completed",
			orchestration: &Orchestration{
				Steps: []OrchestrationStep{
					{
						Activities: []Activity{
							{ID: "act1", Type: "test"},
							{ID: "act2", Type: "test"},
							{ID: "act3", Type: "test"},
						},
					},
				},
				Completed: map[string]struct{}{"act1": {}, "act2": {}, "act3": {}},
			},
			activityID: "act3",
			want:       true, // no next step but the orchestration can proceed, i.e. it is finished
			wantErr:    false,
		},
		{
			name: "step with pending activities",
			orchestration: &Orchestration{
				Steps: []OrchestrationStep{
					{
						Activities: []Activity{
							{ID: "act1", Type: "test"},
							{ID: "act2", Type: "test"},
							{ID: "act3", Type: "test"},
						},
					},
				},
			},
			activityID: "act1",
			want:       false,
			wantErr:    false,
		},
		{
			name: "activity not found",
			orchestration: &Orchestration{
				Steps: []OrchestrationStep{
					{
						Activities: []Activity{
							{ID: "act1", Type: "test"},
						},
					},
				},
			},
			activityID: "non-existent",
			want:       false, // Should return false when activity not found
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.orchestration.CanProceedToNextStep(tt.activityID)
			if (err != nil) != tt.wantErr {
				t.Errorf("CanProceedToNextStep() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CanProceedToNextStep() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStepForActivity(t *testing.T) {
	t.Run("single step orchestration - activity found", func(t *testing.T) {
		// Setup
		activity := Activity{ID: "activity1"}
		orchestration := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{activity},
				},
			},
		}

		step, err := orchestration.GetStepForActivity("activity1")

		require.NoError(t, err)
		require.NotNil(t, step)
		require.Equal(t, activity, step.Activities[0])
	})

	t.Run("two step orchestration - activity found in second step", func(t *testing.T) {
		// Setup
		activity1 := Activity{ID: "activity1"}
		activity2 := Activity{ID: "activity2"}
		orchestration := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{activity1},
				},
				{
					Activities: []Activity{activity2},
				},
			},
		}

		step, err := orchestration.GetStepForActivity("activity2")

		// Assert
		require.NoError(t, err)
		require.NotNil(t, step)
		require.Equal(t, activity2, step.Activities[0])
	})

	t.Run("activity not found", func(t *testing.T) {
		// Setup
		activity := Activity{ID: "activity1"}
		orchestration := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{activity},
				},
			},
		}

		step, err := orchestration.GetStepForActivity("nonexistent")

		require.Error(t, err)
		require.Nil(t, step)
		require.Contains(t, err.Error(), "step not found for activity: nonexistent")
	})

	t.Run("empty orchestration", func(t *testing.T) {
		// Setup
		orchestration := &Orchestration{
			Steps: []OrchestrationStep{},
		}

		step, err := orchestration.GetStepForActivity("activity1")

		require.Error(t, err)
		require.Nil(t, step)
		require.Contains(t, err.Error(), "step not found for activity: activity1")
	})
}

func TestGetNextActivities(t *testing.T) {
	t.Run("single step with single activity - no next activities", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
			},
		}

		activities := orch.GetNextStepActivities("a1")
		require.Empty(t, activities)
	})

	t.Run("single step with multiple parallel activities - no next activities", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
						{ID: "a2", Type: "test"},
						{ID: "a3", Type: "test"},
					},
				},
			},
		}

		// Test each activity in the single step
		activities := orch.GetNextStepActivities("a1")
		require.Empty(t, activities)

		activities = orch.GetNextStepActivities("a2")
		require.Empty(t, activities)

		activities = orch.GetNextStepActivities("a3")
		require.Empty(t, activities)
	})

	t.Run("two steps - single to single", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "b1", Type: "test"},
					},
				},
			},
		}

		// Activity in first step should return next step's activity
		activities := orch.GetNextStepActivities("a1")
		require.Len(t, activities, 1)
		require.Equal(t, "b1", activities[0].ID)

		// Activity in last step should return empty
		activities = orch.GetNextStepActivities("b1")
		require.Empty(t, activities)
	})

	t.Run("two steps - single to multiple parallel", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "b1", Type: "test"},
						{ID: "b2", Type: "test"},
						{ID: "b3", Type: "test"},
					},
				},
			},
		}

		activities := orch.GetNextStepActivities("a1")
		require.Len(t, activities, 3)

		// Verify all next step activities are returned
		activityIDs := make([]string, len(activities))
		for i, act := range activities {
			activityIDs[i] = act.ID
		}
		require.Contains(t, activityIDs, "b1")
		require.Contains(t, activityIDs, "b2")
		require.Contains(t, activityIDs, "b3")
	})

	t.Run("two steps - multiple parallel to single", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
						{ID: "a2", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "b1", Type: "test"},
					},
				},
			},
		}

		// Both activities in first step should return the same next step activity
		activities := orch.GetNextStepActivities("a1")
		require.Len(t, activities, 1)
		require.Equal(t, "b1", activities[0].ID)

		activities = orch.GetNextStepActivities("a2")
		require.Len(t, activities, 1)
		require.Equal(t, "b1", activities[0].ID)
	})

	t.Run("two steps - multiple parallel to multiple parallel", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
						{ID: "a2", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "b1", Type: "test"},
						{ID: "b2", Type: "test"},
					},
				},
			},
		}

		// All activities in first step should return all activities from next step
		for _, actID := range []string{"a1", "a2"} {
			activities := orch.GetNextStepActivities(actID)
			require.Len(t, activities, 2)

			activityIDs := make([]string, len(activities))
			for i, act := range activities {
				activityIDs[i] = act.ID
			}
			require.Contains(t, activityIDs, "b1")
			require.Contains(t, activityIDs, "b2")
		}
	})

	t.Run("three steps - complex progression", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "b1", Type: "test"},
						{ID: "b2", Type: "test"},
						{ID: "b3", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "c1", Type: "test"},
					},
				},
			},
		}

		// First step activity should return second step activities
		activities := orch.GetNextStepActivities("a1")
		require.Len(t, activities, 3)
		activityIDs := make([]string, len(activities))
		for i, act := range activities {
			activityIDs[i] = act.ID
		}
		require.Contains(t, activityIDs, "b1")
		require.Contains(t, activityIDs, "b2")
		require.Contains(t, activityIDs, "b3")

		// Second step activities should return third step activity
		for _, actID := range []string{"b1", "b2", "b3"} {
			activities = orch.GetNextStepActivities(actID)
			require.Len(t, activities, 1)
			require.Equal(t, "c1", activities[0].ID)
		}

		// Last step activity should return empty
		activities = orch.GetNextStepActivities("c1")
		require.Empty(t, activities)
	})

	t.Run("four steps - alternating single and parallel", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "b1", Type: "test"},
						{ID: "b2", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "c1", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "d1", Type: "test"},
						{ID: "d2", Type: "test"},
						{ID: "d3", Type: "test"},
					},
				},
			},
		}

		// Test progression through all steps
		activities := orch.GetNextStepActivities("a1")
		require.Len(t, activities, 2)

		activities = orch.GetNextStepActivities("b1")
		require.Len(t, activities, 1)
		require.Equal(t, "c1", activities[0].ID)

		activities = orch.GetNextStepActivities("c1")
		require.Len(t, activities, 3)

		activities = orch.GetNextStepActivities("d1")
		require.Empty(t, activities)
	})

	t.Run("empty step in sequence", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
				{
					Activities: []Activity{}, // Empty step
				},
				{
					Activities: []Activity{
						{ID: "c1", Type: "test"},
					},
				},
			},
		}

		// Activity before empty step should return empty step activities (which is empty)
		activities := orch.GetNextStepActivities("a1")
		require.Empty(t, activities)
	})

	t.Run("multiple empty steps", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
				{
					Activities: []Activity{}, // Empty step
				},
				{
					Activities: []Activity{}, // Another empty step
				},
				{
					Activities: []Activity{
						{ID: "d1", Type: "test"},
					},
				},
			},
		}

		// Should return the immediate next step (even if empty)
		activities := orch.GetNextStepActivities("a1")
		require.Empty(t, activities)
	})

	t.Run("non-existent activity", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{ID: "a1", Type: "test"},
					},
				},
				{
					Activities: []Activity{
						{ID: "b1", Type: "test"},
					},
				},
			},
		}

		activities := orch.GetNextStepActivities("non-existent")
		require.Empty(t, activities)
	})

	t.Run("empty orchestration", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{},
		}

		activities := orch.GetNextStepActivities("any-activity")
		require.Empty(t, activities)
	})

	t.Run("orchestration with only empty steps", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{Activities: []Activity{}},
				{Activities: []Activity{}},
				{Activities: []Activity{}},
			},
		}

		activities := orch.GetNextStepActivities("any-activity")
		require.Empty(t, activities)
	})

	t.Run("large parallel step followed by single activity", func(t *testing.T) {
		// Create a large parallel step
		largeParallelActivities := make([]Activity, 10)
		for i := 0; i < 10; i++ {
			largeParallelActivities[i] = Activity{
				ID:   fmt.Sprintf("parallel_%d", i),
				Type: "test",
			}
		}

		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: largeParallelActivities,
				},
				{
					Activities: []Activity{
						{ID: "final", Type: "test"},
					},
				},
			},
		}

		// Test a few activities from the large parallel step
		for i := 0; i < 3; i++ {
			activities := orch.GetNextStepActivities(fmt.Sprintf("parallel_%d", i))
			require.Len(t, activities, 1)
			require.Equal(t, "final", activities[0].ID)
		}
	})

	t.Run("activity with complex metadata", func(t *testing.T) {
		orch := &Orchestration{
			Steps: []OrchestrationStep{
				{
					Activities: []Activity{
						{
							ID:        "complex1",
							Type:      "complex.test.com",
							DependsOn: []string{"dependency1", "dependency2"},
						},
					},
				},
				{
					Activities: []Activity{
						{
							ID:   "complex2",
							Type: "another.complex.test.com",
						},
					},
				},
			},
		}

		activities := orch.GetNextStepActivities("complex1")
		require.Len(t, activities, 1)
		require.Equal(t, "complex2", activities[0].ID)
		require.Equal(t, "another.complex.test.com", activities[0].Type.String())
	})
}

func TestMappingEntry_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    MappingEntry
		wantErr bool
	}{
		{
			name: "string input",
			json: `"test_value"`,
			want: MappingEntry{
				Source: "test_value",
				Target: "test_value",
			},
			wantErr: false,
		},
		{
			name: "object input",
			json: `{"source": "src_value", "target": "tgt_value"}`,
			want: MappingEntry{
				Source: "src_value",
				Target: "tgt_value",
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			json:    `{"invalid`,
			want:    MappingEntry{},
			wantErr: true,
		},
		{
			name:    "invalid type",
			json:    `42`,
			want:    MappingEntry{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got MappingEntry
			err := json.Unmarshal([]byte(tt.json), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("MappingEntry.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.DeepEqual(t, got, tt.want)
		})
	}
}
