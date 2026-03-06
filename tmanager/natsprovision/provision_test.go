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

package natsprovision

import (
	"context"
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvisionCallbackService_RegisterProvisionHandler(t *testing.T) {

	t.Run("register handler", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		handler := func(ctx context.Context, response model.OrchestrationResponse) error {
			return nil
		}

		service.Register("test-type", handler)

		require.Contains(t, service.handlers, "test-type")
		assert.NotNil(t, service.handlers["test-type"])
	})

	t.Run("register multiple handlers", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		handler1 := func(ctx context.Context, response model.OrchestrationResponse) error {
			return nil
		}
		handler2 := func(ctx context.Context, response model.OrchestrationResponse) error {
			return types.NewClientError("test error")
		}

		service.Register("type1", handler1)
		service.Register("type2", handler2)

		require.Contains(t, service.handlers, "type1")
		require.Contains(t, service.handlers, "type2")
		assert.Len(t, service.handlers, 2)
	})

	t.Run("overwrite existing handler", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		originalHandler := func(ctx context.Context, response model.OrchestrationResponse) error {
			return types.NewClientError("original")
		}
		newHandler := func(ctx context.Context, response model.OrchestrationResponse) error {
			return types.NewClientError("new")
		}

		service.Register("test-type", originalHandler)
		service.Register("test-type", newHandler)

		require.Contains(t, service.handlers, "test-type")
		assert.Len(t, service.handlers, 1)
	})
}

func TestProvisionCallbackService_Dispatch(t *testing.T) {
	t.Run("dispatch to registered handler", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		var receivedResponse model.OrchestrationResponse
		handler := func(ctx context.Context, response model.OrchestrationResponse) error {
			receivedResponse = response
			return nil
		}

		service.Register("vpa", handler)

		response := model.OrchestrationResponse{
			Success:           true,
			ErrorDetail:       "",
			ManifestID:        "manifest-123",
			OrchestrationType: "vpa",
			Properties: map[string]any{
				"version": "1.0.0",
			},
		}

		err := service.Dispatch(context.Background(), response)

		require.NoError(t, err)
		assert.Equal(t, response, receivedResponse)
	})

	t.Run("dispatch with handler returning error", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		expectedError := types.NewClientError("orchestration failed")
		handler := func(ctx context.Context, response model.OrchestrationResponse) error {
			return expectedError
		}

		service.Register("vpa", handler)

		response := model.OrchestrationResponse{
			Success:           false,
			ErrorDetail:       "connection timeout",
			OrchestrationType: "vpa",
		}

		err := service.Dispatch(context.Background(), response)

		require.Error(t, err)
		assert.Equal(t, expectedError, err)
	})

	t.Run("dispatch to unregistered orchestration type", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		response := model.OrchestrationResponse{
			OrchestrationType: "nonexistent-type",
		}

		err := service.Dispatch(context.Background(), response)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "provision handler not found for type: nonexistent-type")

		require.True(t, types.IsFatal(err))

	})

	t.Run("dispatch with context cancellation", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		handler := func(ctx context.Context, response model.OrchestrationResponse) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		}

		service.Register("vpa", handler)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		response := model.OrchestrationResponse{
			OrchestrationType: "vpa",
		}

		err := service.Dispatch(ctx, response)

		require.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

}

func TestProvisionCallbackService_Integration(t *testing.T) {
	t.Run("multiple handlers", func(t *testing.T) {
		service := provisionCallbackService{
			handlers: make(map[string]api.ProvisionCallbackHandler),
		}

		var connectorCalls int
		var dataspaceCalls int

		connectorHandler := func(ctx context.Context, response model.OrchestrationResponse) error {
			connectorCalls++
			return nil
		}

		dataspaceHandler := func(ctx context.Context, response model.OrchestrationResponse) error {
			dataspaceCalls++
			return types.NewRecoverableError("temporary failure")
		}

		service.Register("vpa", connectorHandler)
		service.Register("dprofile", dataspaceHandler)

		connectorResponse := model.OrchestrationResponse{OrchestrationType: "vpa"}
		err := service.Dispatch(context.Background(), connectorResponse)
		require.NoError(t, err)

		dataspaceResponse := model.OrchestrationResponse{OrchestrationType: "dprofile"}
		err = service.Dispatch(context.Background(), dataspaceResponse)
		require.Error(t, err)

		// Verify only correct handlers were called
		assert.Equal(t, 1, connectorCalls)
		assert.Equal(t, 1, dataspaceCalls)

		require.True(t, types.IsRecoverable(err))
	})

}
