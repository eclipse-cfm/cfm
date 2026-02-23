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
	"fmt"
	"testing"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/stretchr/testify/assert"
)

func TestInstantiateOrchestration(t *testing.T) {
	t.Run("successful instantiation with no dependencies", func(t *testing.T) {
		orchestrationID := "test-orchestration-123"
		definition := []Activity{
			{
				ID:        "activity1",
				Type:      "test-type",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
			{
				ID:        "activity2",
				Type:      "test-type-2",
				Inputs:    []MappingEntry{{Source: "input2", Target: "target2"}},
				DependsOn: []string{},
			},
		}
		data := map[string]any{
			"key1": "value1",
			"key2": 42,
		}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-definition-id", definition, data)

		assert.NoError(t, err)
		assert.NotNil(t, orchestration)
		assert.NotEmpty(t, orchestration.ID)
		assert.Equal(t, OrchestrationStateInitialized, orchestration.State)
		assert.Equal(t, data, orchestration.ProcessingData)
		assert.NotNil(t, orchestration.Completed)
		assert.Equal(t, 0, len(orchestration.Completed))
		assert.True(t, len(orchestration.Steps) > 0)

		// Verify activity order: activities without dependencies should be in the first step
		assert.Equal(t, 1, len(orchestration.Steps))
		assert.Equal(t, 2, len(orchestration.Steps[0].Activities))
		activityIDs := make([]string, len(orchestration.Steps[0].Activities))
		for i, activity := range orchestration.Steps[0].Activities {
			activityIDs[i] = activity.ID
		}
		assert.Contains(t, activityIDs, "activity1")
		assert.Contains(t, activityIDs, "activity2")

	})

	t.Run("successful instantiation with linear dependencies", func(t *testing.T) {
		orchestrationID := "test-orchestration-linear"
		definition := []Activity{
			{
				ID:        "activity1",
				Type:      "first",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
			{
				ID:        "activity2",
				Type:      "second",
				Inputs:    []MappingEntry{{Source: "input2", Target: "target2"}},
				DependsOn: []string{"activity1"},
			},
			{
				ID:        "activity3",
				Type:      "third",
				Inputs:    []MappingEntry{{Source: "input3", Target: "target3"}},
				DependsOn: []string{"activity2"},
			},
		}
		data := map[string]any{"test": "data"}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "some-definition-id", definition, data)

		assert.NoError(t, err)
		assert.NotNil(t, orchestration)
		assert.Equal(t, OrchestrationStateInitialized, orchestration.State)

		// Verify activities are present in the orchestration
		allActivities := make(map[string]bool)
		for _, step := range orchestration.Steps {
			for _, activity := range step.Activities {
				allActivities[activity.ID] = true
			}
		}
		assert.True(t, allActivities["activity1"])
		assert.True(t, allActivities["activity2"])
		assert.True(t, allActivities["activity3"])

		// Verify activity order: linear dependencies should create separate steps
		assert.Equal(t, 3, len(orchestration.Steps))
		assert.Equal(t, "activity1", orchestration.Steps[0].Activities[0].ID)
		assert.Equal(t, "activity2", orchestration.Steps[1].Activities[0].ID)
		assert.Equal(t, "activity3", orchestration.Steps[2].Activities[0].ID)

	})

	t.Run("successful instantiation with parallel dependencies", func(t *testing.T) {
		orchestrationID := "test-orchestration-parallel"
		definition := []Activity{
			{
				ID:        "activity1",
				Type:      "root",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
			{
				ID:        "activity2",
				Type:      "parallel1",
				Inputs:    []MappingEntry{{Source: "input2", Target: "target2"}},
				DependsOn: []string{"activity1"},
			},
			{
				ID:        "activity3",
				Type:      "parallel2",
				Inputs:    []MappingEntry{{Source: "input3", Target: "target3"}},
				DependsOn: []string{"activity1"},
			},
			{
				ID:        "activity4",
				Type:      "final",
				Inputs:    []MappingEntry{{Source: "input4", Target: "target4"}},
				DependsOn: []string{"activity2", "activity3"},
			},
		}
		data := map[string]any{"parallel": "test"}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-definition-id", definition, data)

		assert.NoError(t, err)
		assert.NotNil(t, orchestration)
		assert.Equal(t, OrchestrationStateInitialized, orchestration.State)
		assert.True(t, len(orchestration.Steps) > 0)

		// Verify activity order: parallel structure should have proper step ordering
		assert.Equal(t, 3, len(orchestration.Steps))

		// Step 1: activity1 (root)
		assert.Equal(t, 1, len(orchestration.Steps[0].Activities))
		assert.Equal(t, "activity1", orchestration.Steps[0].Activities[0].ID)

		// Step 2: activity2 and activity3 (parallel)
		assert.Equal(t, 2, len(orchestration.Steps[1].Activities))
		parallelActivityIDs := make([]string, 2)
		for i, activity := range orchestration.Steps[1].Activities {
			parallelActivityIDs[i] = activity.ID
		}
		assert.Contains(t, parallelActivityIDs, "activity2")
		assert.Contains(t, parallelActivityIDs, "activity3")

		// Step 3: activity4 (final)
		assert.Equal(t, 1, len(orchestration.Steps[2].Activities))
		assert.Equal(t, "activity4", orchestration.Steps[2].Activities[0].ID)

	})

	t.Run("error when cycle detected", func(t *testing.T) {
		orchestrationID := "test-orchestration-cycle"
		activities := []Activity{
			{
				ID:        "activity1",
				Type:      "first",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{"activity2"},
			},
			{
				ID:        "activity2",
				Type:      "second",
				Inputs:    []MappingEntry{{Source: "input2", Target: "target2"}},
				DependsOn: []string{"activity1"},
			},
		}
		data := map[string]any{"cycle": "test"}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-definition-id", activities, data)

		assert.Error(t, err)
		assert.Nil(t, orchestration)
		assert.Contains(t, err.Error(), "cycle detected")
	})

	t.Run("successful instantiation with empty definition", func(t *testing.T) {
		orchestrationID := "test-orchestration-empty"
		var activities []Activity
		data := map[string]any{"empty": "test"}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-activities-id", activities, data)

		assert.NoError(t, err)
		assert.NotNil(t, orchestration)
		assert.Equal(t, OrchestrationStateInitialized, orchestration.State)
		assert.Equal(t, data, orchestration.ProcessingData)
		assert.Equal(t, 0, len(orchestration.Steps))
	})

	t.Run("successful instantiation with nil data", func(t *testing.T) {
		orchestrationID := "test-orchestration-nil-data"
		activities := []Activity{
			{
				ID:        "activity1",
				Type:      "test",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
		}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-definition-id", activities, nil)

		assert.NoError(t, err)
		assert.NotNil(t, orchestration)
		assert.Equal(t, OrchestrationStateInitialized, orchestration.State)
		assert.NotNil(t, orchestration.ProcessingData)
		assert.NotNil(t, orchestration.Completed)

		// Verify activity order: single activity should be in first step
		assert.Equal(t, 1, len(orchestration.Steps))
		assert.Equal(t, 1, len(orchestration.Steps[0].Activities))
		assert.Equal(t, "activity1", orchestration.Steps[0].Activities[0].ID)
	})

	t.Run("successful instantiation with complex dependencies", func(t *testing.T) {
		orchestrationID := "test-orchestration-complex"
		activities := []Activity{
			{
				ID:        "activity1",
				Type:      "init",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
			{
				ID:        "activity2",
				Type:      "setup",
				Inputs:    []MappingEntry{{Source: "input2", Target: "target2"}},
				DependsOn: []string{},
			},
			{
				ID:        "activity3",
				Type:      "process",
				Inputs:    []MappingEntry{{Source: "input3", Target: "target3"}},
				DependsOn: []string{"activity1", "activity2"},
			},
			{
				ID:        "activity4",
				Type:      "validate",
				Inputs:    []MappingEntry{{Source: "input4", Target: "target4"}},
				DependsOn: []string{"activity3"},
			},
			{
				ID:        "activity5",
				Type:      "cleanup",
				Inputs:    []MappingEntry{{Source: "input5", Target: "target5"}},
				DependsOn: []string{"activity4"},
			},
		}
		data := map[string]any{
			"config": map[string]any{
				"timeout": 30,
				"retries": 3,
			},
		}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-activities-id", activities, data)

		assert.NoError(t, err)
		assert.NotNil(t, orchestration)
		assert.Equal(t, OrchestrationStateInitialized, orchestration.State)
		assert.Equal(t, data, orchestration.ProcessingData)

		// Verify all activities are included
		allActivities := make(map[string]bool)
		for _, step := range orchestration.Steps {
			for _, activity := range step.Activities {
				allActivities[activity.ID] = true
			}
		}
		assert.Equal(t, 5, len(allActivities))
		for i := 1; i <= 5; i++ {
			activityID := fmt.Sprintf("activity%d", i)
			assert.True(t, allActivities[activityID], "Activity %s should be present", activityID)
		}

		// Verify activity order: complex dependencies should create proper step structure
		assert.Equal(t, 4, len(orchestration.Steps))

		// Step 1: activity1 and activity2 (no dependencies)
		assert.Equal(t, 2, len(orchestration.Steps[0].Activities))
		step1ActivityIDs := make([]string, 2)
		for i, activity := range orchestration.Steps[0].Activities {
			step1ActivityIDs[i] = activity.ID
		}
		assert.Contains(t, step1ActivityIDs, "activity1")
		assert.Contains(t, step1ActivityIDs, "activity2")

		// Step 2: activity3 (depends on activity1 and activity2)
		assert.Equal(t, 1, len(orchestration.Steps[1].Activities))
		assert.Equal(t, "activity3", orchestration.Steps[1].Activities[0].ID)

		// Step 3: activity4 (depends on activity3)
		assert.Equal(t, 1, len(orchestration.Steps[2].Activities))
		assert.Equal(t, "activity4", orchestration.Steps[2].Activities[0].ID)

		// Step 4: activity5 (depends on activity4)
		assert.Equal(t, 1, len(orchestration.Steps[3].Activities))
		assert.Equal(t, "activity5", orchestration.Steps[3].Activities[0].ID)
	})

	t.Run("uses orchestration ID", func(t *testing.T) {
		orchestrationID := "test-orchestration-id"
		activities := []Activity{
			{
				ID:        "activity1",
				Type:      "test",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{},
			},
		}
		data := map[string]any{"test": "data"}

		orchestration, err1 := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-activities-id", activities, data)

		assert.NoError(t, err1)
		assert.NotNil(t, orchestration)
		assert.Equal(t, orchestration.ID, orchestrationID)
	})

	t.Run("error with invalid dependency reference", func(t *testing.T) {
		orchestrationID := "test-orchestration-invalid-dep"
		activities := []Activity{
			{
				ID:        "activity1",
				Type:      "test",
				Inputs:    []MappingEntry{{Source: "input1", Target: "target1"}},
				DependsOn: []string{"non-existent-activity"},
			},
		}
		data := map[string]any{"test": "data"}

		orchestration, err := InstantiateOrchestration(orchestrationID, "123", model.VPADeployType, "test-activities-id", activities, data)

		assert.Error(t, err)
		assert.Nil(t, orchestration)
		assert.Contains(t, err.Error(), "dependency not found")
	})
}
