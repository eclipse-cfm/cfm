/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package vpa

import (
	"context"
	"testing"

	"github.com/eclipse-cfm/cfm/common/dataspace"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testActivity = api.Activity{ID: "test-activity", Type: "test", Discriminator: DeployDiscriminator}

// recordingProcessor records which of its methods was called and returns a result that identifies the method.
type recordingProcessor struct {
	called []string
}

func (p *recordingProcessor) ProcessDeploy(api.ActivityContext) api.ActivityResult {
	p.called = append(p.called, "deploy")
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (p *recordingProcessor) ProcessDispose(api.ActivityContext) api.ActivityResult {
	p.called = append(p.called, "dispose")
	return api.ActivityResult{Result: api.ActivityResultSchedule}
}

func newContext(discriminator api.Discriminator) api.ActivityContext {
	activity := api.Activity{ID: "A1", Type: "test", Discriminator: discriminator}
	return api.NewActivityContext(context.Background(), "orch-1", activity, map[string]any{}, map[string]any{})
}

func TestAsActivityProcessor_DeployDiscriminatorCallsProcessDeploy(t *testing.T) {
	processor := &recordingProcessor{}

	result := AsActivityProcessor(processor).Process(newContext(DeployDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultComplete), result.Result)
	assert.Equal(t, []string{"deploy"}, processor.called)
}

func TestAsActivityProcessor_DisposeDiscriminatorCallsProcessDispose(t *testing.T) {
	processor := &recordingProcessor{}

	result := AsActivityProcessor(processor).Process(newContext(DisposeDiscriminator))

	assert.Equal(t, api.ActivityResultType(api.ActivityResultSchedule), result.Result)
	assert.Equal(t, []string{"dispose"}, processor.called)
}

func TestAsActivityProcessor_OtherDiscriminatorIsFatal(t *testing.T) {
	for _, discriminator := range []api.Discriminator{"", "cfm.orchestration.other"} {
		t.Run(string(discriminator), func(t *testing.T) {
			processor := &recordingProcessor{}

			result := AsActivityProcessor(processor).Process(newContext(discriminator))

			assert.Equal(t, api.ActivityResultType(api.ActivityResultFatalError), result.Result)
			require.Error(t, result.Error)
			assert.ErrorContains(t, result.Error, "not supported")
			assert.Empty(t, processor.called)
		})
	}
}

const testVpaType dataspace.VPAType = "cfm.test.vpa.type"
const otherVpaType dataspace.VPAType = "cfm.test.vpa.other"

func TestProperties_NoEntryForVpa(t *testing.T) {
	props := map[string]any{"foo": "bar", "quizz": "quazz"}
	processingData := map[string]any{
		dataspace.VPAData: []any{
			map[string]any{
				"vpaType":    testVpaType.String(),
				"properties": props,
			},
		},
	}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	found, err := Properties(ctx, "some-other-type")

	require.NoError(t, err)
	require.Empty(t, found)
}

func TestProperties_NilValue(t *testing.T) {
	processingData := map[string]any{dataspace.VPAData: nil}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	data, err := Properties(ctx, testVpaType)

	require.NoError(t, err)
	require.Nil(t, data)
}

func TestProperties_NotASlice(t *testing.T) {
	processingData := map[string]any{dataspace.VPAData: "not-a-slice"}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	_, err := Properties(ctx, testVpaType)

	require.Error(t, err)
	assert.ErrorContains(t, err, "not a slice")
}

func TestProperties_EmptySlice(t *testing.T) {
	processingData := map[string]any{dataspace.VPAData: []any{}}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	data, err := Properties(ctx, testVpaType)

	require.NoError(t, err)
	require.Empty(t, data)
}

func TestProperties_NoMatchingType(t *testing.T) {
	processingData := map[string]any{
		dataspace.VPAData: []any{
			map[string]any{"vpaType": otherVpaType.String()},
		},
	}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	data, err := Properties(ctx, testVpaType)

	require.NoError(t, err)
	require.Empty(t, data)
}

func TestProperties_MatchingTypeWithProperties(t *testing.T) {
	props := map[string]any{"region": "eu-west", "tier": "standard"}
	processingData := map[string]any{
		dataspace.VPAData: []any{
			map[string]any{
				"vpaType":    testVpaType.String(),
				"properties": props,
			},
		},
	}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	result, err := Properties(ctx, testVpaType)

	require.NoError(t, err)
	assert.Equal(t, props, result)
}

func TestProperties_MatchingTypeNotFirst(t *testing.T) {
	props := map[string]any{"region": "eu-west", "tier": "standard"}
	processingData := map[string]any{
		dataspace.VPAData: []any{
			map[string]any{"vpaType": otherVpaType.String()},
			map[string]any{
				"vpaType":    testVpaType.String(),
				"properties": props,
			},
		},
	}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	result, err := Properties(ctx, testVpaType)

	require.NoError(t, err)
	assert.Equal(t, props, result)
}

func TestProperties_MatchingTypeWithoutProperties(t *testing.T) {
	processingData := map[string]any{
		dataspace.VPAData: []any{
			map[string]any{"vpaType": testVpaType.String()},
		},
	}
	ctx := api.NewActivityContext(context.Background(), "orch-1", testActivity, processingData, map[string]any{})

	result, err := Properties(ctx, testVpaType)

	require.NoError(t, err)
	assert.Empty(t, result)
}
