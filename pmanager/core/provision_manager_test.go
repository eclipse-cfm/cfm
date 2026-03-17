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

package core

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cmocks "github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/memorystore"
	"github.com/eclipse-cfm/cfm/pmanager/mocks"
)

func TestProvisionManager_Start(t *testing.T) {
	tests := []struct {
		name           string
		manifest       *model.OrchestrationManifest
		setupStore     func(store api.DefinitionStore)
		setupOrch      func(orch *mocks.MockOrchestrator)
		expectedError  string
		expectedResult *api.Orchestration
	}{
		{
			name: "successful deployment with new orchestration",
			manifest: &model.OrchestrationManifest{
				ID:                "test-orchestration-1",
				OrchestrationType: "test-type",
				Payload:           map[string]any{"key": "value"},
			},
			setupStore: func(store api.DefinitionStore) {
				definition := &api.OrchestrationDefinition{
					Type: "test-type",
					Activities: []api.Activity{
						{
							ID:   "activity1",
							Type: "test-activity",
						},
					},
				}
				ctx := context.Background()
				_, _ = store.StoreOrchestrationDefinition(ctx, definition)
			},
			setupOrch: func(orch *mocks.MockOrchestrator) {
				orch.EXPECT().GetOrchestration(mock.Anything, "test-orchestration-1").Return(nil, nil)
				orch.EXPECT().Execute(mock.Anything, mock.AnythingOfType("*api.Orchestration")).Return(nil)
			},
			expectedResult: &api.Orchestration{
				ID: "test-orchestration-1",
			},
		},
		{
			name: "deduplication - successful deployment with existing orchestration",
			manifest: &model.OrchestrationManifest{
				ID:                "test-orchestration-2",
				OrchestrationType: "test-type",
				Payload:           map[string]any{"key": "value"},
			},
			setupStore: func(store api.DefinitionStore) {
				definition := &api.OrchestrationDefinition{
					Type: "test-type",
					Activities: []api.Activity{
						{
							ID:   "activity1",
							Type: "test-activity",
						},
					},
				}
				ctx := context.Background()
				_, _ = store.StoreOrchestrationDefinition(ctx, definition)
			},
			setupOrch: func(orch *mocks.MockOrchestrator) {
				existingOrch := &api.Orchestration{
					ID: "test-orchestration-2",
				}
				orch.EXPECT().GetOrchestration(mock.Anything, "test-orchestration-2").Return(existingOrch, nil)
			},
			expectedResult: &api.Orchestration{
				ID: "test-orchestration-2",
			},
		},
		{
			name: "orchestration definition not found",
			manifest: &model.OrchestrationManifest{
				ID:                "test-orchestration-3",
				OrchestrationType: "non-existent-type",
				Payload:           map[string]any{"key": "value"},
			},
			setupStore: func(store api.DefinitionStore) {
				// Don't store any definitions
			},
			setupOrch: func(orch *mocks.MockOrchestrator) {
				// No orchestrator calls expected
			},
			expectedError: "orchestration type 'non-existent-type' not found",
		},
		{
			name: "orchestrator get orchestration error",
			manifest: &model.OrchestrationManifest{
				ID:                "test-orchestration-5",
				OrchestrationType: "test-type",
				Payload:           map[string]any{"key": "value"},
			},
			setupStore: func(store api.DefinitionStore) {
				definition := &api.OrchestrationDefinition{
					Type: "test-type",
					Activities: []api.Activity{
						{
							ID:   "activity1",
							Type: "test-activity",
						},
					},
				}
				ctx := context.Background()
				_, _ = store.StoreOrchestrationDefinition(ctx, definition)
			},
			setupOrch: func(orch *mocks.MockOrchestrator) {
				orch.EXPECT().GetOrchestration(mock.Anything, "test-orchestration-5").Return(nil, errors.New("orchestrator error"))
			},
			expectedError: "error performing de-duplication for test-orchestration-5",
		},
		{
			name: "orchestrator execute orchestration error",
			manifest: &model.OrchestrationManifest{
				ID:                "test-orchestration-6",
				OrchestrationType: "test-type",
				Payload:           map[string]any{"key": "value"},
			},
			setupStore: func(store api.DefinitionStore) {
				definition := &api.OrchestrationDefinition{
					Type: "test-type",
					Activities: []api.Activity{
						{
							ID:   "activity1",
							Type: "test-activity",
						},
					},
				}
				ctx := context.Background()
				_, _ = store.StoreOrchestrationDefinition(ctx, definition)
			},
			setupOrch: func(orch *mocks.MockOrchestrator) {
				orch.EXPECT().GetOrchestration(mock.Anything, "test-orchestration-6").Return(nil, nil)
				orch.EXPECT().Execute(mock.Anything, mock.AnythingOfType("*api.Orchestration")).Return(errors.New("execution error"))
			},
			expectedError: "error executing orchestration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup memory store
			definitionStore := memorystore.NewDefinitionStore()
			tt.setupStore(definitionStore)

			// Setup mock orchestrator
			mockOrch := mocks.NewMockOrchestrator(t)
			tt.setupOrch(mockOrch)

			// Create provision manager
			pm := &provisionManager{
				orchestrator: mockOrch,
				store:        definitionStore,
				monitor:      &system.NoopMonitor{},
				trxContext:   store.NoOpTransactionContext{},
			}

			// Execute test
			ctx := context.Background()
			result, err := pm.Start(ctx, tt.manifest)

			// Assert results
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedResult.ID, result.ID)
				//if tt.expectedResult.Status != "" {
				//	assert.Equal(t, tt.expectedResult.Status, result.Status)
				//}
			}
		})
	}
}

// Test helper to verify orchestration instantiation
func TestProvisionManager_Start_OrchestrationInstantiation(t *testing.T) {
	// Setup memory store with test definition
	definitionStore := memorystore.NewDefinitionStore()
	definition := createTestOrchestrationDefinition("test-type")

	ctx := context.Background()
	_, _ = definitionStore.StoreOrchestrationDefinition(ctx, definition)

	// Setup mock orchestrator
	mockOrch := mocks.NewMockOrchestrator(t)
	mockOrch.EXPECT().GetOrchestration(mock.Anything, "test-deployment").Return(nil, nil)
	mockOrch.EXPECT().Execute(mock.Anything, mock.MatchedBy(func(orch *api.Orchestration) bool {
		// Verify orchestration properties
		return orch.ID == "test-deployment"
	})).Return(nil)

	// Create provision manager
	pm := &provisionManager{
		orchestrator: mockOrch,
		store:        definitionStore,
		monitor:      &system.NoopMonitor{},
		trxContext:   store.NoOpTransactionContext{},
	}

	// Create test manifest
	manifest := &model.OrchestrationManifest{
		ID:                "test-deployment",
		OrchestrationType: "test-type",
		Payload:           map[string]any{"key": "value"},
	}

	// Execute test
	result, err := pm.Start(ctx, manifest)

	// Assert results
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-deployment", result.ID)
}

// TestCountOrchestrations_WithEmptyResult tests counting with no matching orchestrations
func TestCountOrchestrations_WithEmptyResult(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	mockEntityStore.On("CountByPredicate", ctx, &query.AtomicPredicate{}).
		Return(int64(0), nil)

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	count, err := pm.CountOrchestrations(ctx, &query.AtomicPredicate{})

	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
	mockEntityStore.AssertExpectations(t)
}

// TestCountOrchestrations_WithResults tests counting multiple matching orchestrations
func TestCountOrchestrations_WithResults(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	predicate := &query.AtomicPredicate{}
	mockEntityStore.On("CountByPredicate", ctx, predicate).
		Return(int64(5), nil)

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	count, err := pm.CountOrchestrations(ctx, predicate)

	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
	mockEntityStore.AssertExpectations(t)
}

// TestCountOrchestrations_WithStorageError tests handling of storage layer errors
func TestCountOrchestrations_WithStorageError(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	expectedErr := types.ErrNotFound

	mockEntityStore.On("CountByPredicate", ctx, &query.AtomicPredicate{}).
		Return(int64(0), expectedErr)

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	count, err := pm.CountOrchestrations(ctx, &query.AtomicPredicate{})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, int64(0), count)
	mockEntityStore.AssertExpectations(t)
}

// TestQueryOrchestrations_WithEmptyResult tests querying with no matching orchestrations
func TestQueryOrchestrations_WithEmptyResult(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	mockEntityStore.On("FindByPredicatePaginated", ctx, &query.AtomicPredicate{}, store.PaginationOptions{}).
		Return(func(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
			return func(yield func(*api.OrchestrationEntry, error) bool) {
				// Return empty sequence
			}
		})

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	predicate := &query.AtomicPredicate{}
	options := store.PaginationOptions{}

	var results []api.OrchestrationEntry
	var errs []error

	for entry, err := range pm.QueryOrchestrations(ctx, predicate, options) {
		results = append(results, *entry)
		if err != nil {
			errs = append(errs, err)
		}
	}

	require.Equal(t, 0, len(errs))
	assert.Equal(t, 0, len(results))
	mockEntityStore.AssertExpectations(t)
}

// TestQueryOrchestrations_WithResults tests querying multiple matching orchestrations
func TestQueryOrchestrations_WithResults(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	expectedEntries := []api.OrchestrationEntry{
		{
			ID:                "orch-1",
			CorrelationID:     "corr-1",
			OrchestrationType: "test-type",
		},
		{
			ID:                "orch-2",
			CorrelationID:     "corr-2",
			OrchestrationType: "test-type",
		},
	}

	mockEntityStore.On("FindByPredicatePaginated", ctx, &query.AtomicPredicate{}, store.PaginationOptions{}).
		Return(func(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
			return func(yield func(*api.OrchestrationEntry, error) bool) {
				for _, entry := range expectedEntries {
					if !yield(&entry, nil) {
						return
					}
				}
			}
		})

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	predicate := &query.AtomicPredicate{}
	options := store.PaginationOptions{}

	var results []api.OrchestrationEntry

	for entry, err := range pm.QueryOrchestrations(ctx, predicate, options) {
		require.NoError(t, err)
		results = append(results, *entry)
	}

	require.Equal(t, 2, len(results))
	assert.Equal(t, "orch-1", results[0].ID)
	assert.Equal(t, "orch-2", results[1].ID)
	mockEntityStore.AssertExpectations(t)
}

// TestQueryOrchestrations_WithStorageError tests handling of storage layer errors
func TestQueryOrchestrations_WithStorageError(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	expectedErr := types.ErrNotFound

	mockEntityStore.On("FindByPredicatePaginated", ctx, &query.AtomicPredicate{}, store.PaginationOptions{}).
		Return(func(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
			return func(yield func(*api.OrchestrationEntry, error) bool) {
				yield(&api.OrchestrationEntry{}, expectedErr)
			}
		})

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	predicate := &query.AtomicPredicate{}
	options := store.PaginationOptions{}

	var errorCount int

	for _, err := range pm.QueryOrchestrations(ctx, predicate, options) {
		if err != nil {
			errorCount++
			require.Equal(t, expectedErr, err)
		}
	}

	require.Equal(t, 1, errorCount)
	mockEntityStore.AssertExpectations(t)
}

// TestQueryOrchestrations_WithPagination tests pagination of results
func TestQueryOrchestrations_WithPagination(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	expectedEntries := []api.OrchestrationEntry{
		{
			ID:                "orch-1",
			CorrelationID:     "corr-1",
			OrchestrationType: "test-type",
		},
		{
			ID:                "orch-2",
			CorrelationID:     "corr-2",
			OrchestrationType: "test-type",
		},
	}

	paginationOptions := store.PaginationOptions{
		Limit:  2,
		Offset: 0,
	}

	mockEntityStore.On("FindByPredicatePaginated", ctx, &query.AtomicPredicate{}, paginationOptions).
		Return(func(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
			return func(yield func(*api.OrchestrationEntry, error) bool) {
				for _, entry := range expectedEntries {
					if !yield(&entry, nil) {
						return
					}
				}
			}
		})

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	predicate := &query.AtomicPredicate{}

	var results []api.OrchestrationEntry

	for entry, err := range pm.QueryOrchestrations(ctx, predicate, paginationOptions) {
		require.NoError(t, err)
		results = append(results, *entry)
	}

	require.Equal(t, 2, len(results))
	mockEntityStore.AssertExpectations(t)
}

// TestQueryOrchestrations_ContextCancellation tests early termination via iterator
func TestQueryOrchestrations_ContextCancellation(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	expectedEntries := []api.OrchestrationEntry{
		{
			ID:                "orch-1",
			CorrelationID:     "corr-1",
			OrchestrationType: "test-type",
		},
		{
			ID:                "orch-2",
			CorrelationID:     "corr-2",
			OrchestrationType: "test-type",
		},
		{
			ID:                "orch-3",
			CorrelationID:     "corr-3",
			OrchestrationType: "test-type",
		},
	}

	mockEntityStore.On("FindByPredicatePaginated", ctx, &query.AtomicPredicate{}, store.PaginationOptions{}).
		Return(func(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
			return func(yield func(*api.OrchestrationEntry, error) bool) {
				for _, entry := range expectedEntries {
					if !yield(&entry, nil) {
						return
					}
				}
			}
		})

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	predicate := &query.AtomicPredicate{}
	options := store.PaginationOptions{}

	var results []api.OrchestrationEntry
	count := 0

	for entry, err := range pm.QueryOrchestrations(ctx, predicate, options) {
		require.NoError(t, err)
		results = append(results, *entry)
		count++
		// Stop after first iteration
		if count >= 1 {
			break
		}
	}

	require.Equal(t, 1, len(results))
	assert.Equal(t, "orch-1", results[0].ID)
}

// TestQueryOrchestrations_ComplexPredicate tests querying with complex predicates
func TestQueryOrchestrations_ComplexPredicate(t *testing.T) {
	ctx := context.Background()
	mockEntityStore := cmocks.NewMockEntityStore[*api.OrchestrationEntry](t)
	trxContext := store.NoOpTransactionContext{}

	// Create a complex predicate (example: AND of multiple conditions)
	complexPredicate := &query.CompoundPredicate{
		Operator: "AND",
		Predicates: []query.Predicate{
			&query.AtomicPredicate{},
		},
	}

	expectedEntries := []api.OrchestrationEntry{
		{
			ID:                "orch-filtered-1",
			CorrelationID:     "corr-1",
			OrchestrationType: "specific-type",
		},
	}

	mockEntityStore.On("FindByPredicatePaginated", ctx, complexPredicate, store.PaginationOptions{}).
		Return(func(ctx context.Context, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
			return func(yield func(*api.OrchestrationEntry, error) bool) {
				for _, entry := range expectedEntries {
					if !yield(&entry, nil) {
						return
					}
				}
			}
		})

	pm := &provisionManager{
		store:      nil,
		index:      mockEntityStore,
		trxContext: trxContext,
	}

	var results []api.OrchestrationEntry

	for entry, err := range pm.QueryOrchestrations(ctx, complexPredicate, store.PaginationOptions{}) {
		require.NoError(t, err)
		results = append(results, *entry)
	}

	require.Equal(t, 1, len(results))
	assert.Equal(t, "orch-filtered-1", results[0].ID)
	assert.Equal(t, model.OrchestrationType("specific-type"), results[0].OrchestrationType)
	mockEntityStore.AssertExpectations(t)
}

// Helper function to create a test orchestration definition
func createTestOrchestrationDefinition(orchestrationType string) *api.OrchestrationDefinition {
	return &api.OrchestrationDefinition{
		Type: model.OrchestrationType(orchestrationType),
		Activities: []api.Activity{
			{
				ID:   "activity1",
				Type: "test-activity",
			},
		},
	}
}
