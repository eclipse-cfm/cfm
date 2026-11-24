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
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"testing"

	"github.com/eclipse-cfm/cfm/common/mocks"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// subjectMsg is a message received on a subject
type subjectMsg struct {
	jetstream.Msg
	subject string
}

func (m subjectMsg) Subject() string {
	return m.subject
}

// dispatch dispatches the manifest as if it was received on the subject
func dispatch(handler *natsProvisionHandler, ctx context.Context, manifest model.OrchestrationManifest, subject string) error {
	return handler.RetriableMessageProcessor.DispatcherCtx(ctx, manifest, subjectMsg{subject: subject})
}

// inNamespace returns the manifest as the handler passes it to the provision manager
func inNamespace(manifest model.OrchestrationManifest, namespace string) *model.OrchestrationManifest {
	manifest.Namespace = namespace
	return &manifest
}

// testOrchestrationType is an orchestration type without meaning to the engine.
const testOrchestrationType model.OrchestrationType = "test.orchestration"

func TestNatsProvisionHandler_Dispatcher_SuccessfulDispatch(t *testing.T) {
	mockClient := mocks.NewMockMsgClient(t)
	mockProvisionManager := &MockProvisionManager{}

	handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})

	ctx := context.Background()
	manifest := model.OrchestrationManifest{
		ID:                "test-manifest-id",
		OrchestrationType: testOrchestrationType,
		Payload:           map[string]any{"key": "value"},
	}

	orchestration := &api.Orchestration{
		ID:    "orchestration-123",
		State: api.OrchestrationStateRunning,
	}

	// Mock expectations - successful provision manager call
	mockProvisionManager.On("Start", ctx, inNamespace(manifest, model.DefaultNamespace)).Return(orchestration, nil)

	// No publish call should be made on success
	err := dispatch(handler, ctx, manifest, natsclient.CFMOrchestrationSubject)

	// Should return no error (message gets ACK'd)
	require.NoError(t, err)

	mockProvisionManager.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestNatsProvisionHandler_Dispatcher_ValidResponseStructure(t *testing.T) {
	mockClient := mocks.NewMockMsgClient(t)
	mockProvisionManager := &MockProvisionManager{}

	handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})

	ctx := context.Background()
	manifest := model.OrchestrationManifest{
		ID:                "test-manifest-id",
		OrchestrationType: testOrchestrationType,
		Payload:           map[string]any{"key": "value"},
	}

	nonRecoverableErr := errors.New("provision failed")
	pubAck := &jetstream.PubAck{}

	// Mock expectations
	mockProvisionManager.On("Start", ctx, inNamespace(manifest, model.DefaultNamespace)).Return(nil, nonRecoverableErr)

	var capturedPayload []byte
	mockClient.EXPECT().Publish(ctx, natsclient.CFMOrchestrationResponseSubject, mock.AnythingOfType("[]uint8")).
		Run(func(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) {
			capturedPayload = payload
		}).Return(pubAck, nil)

	err := dispatch(handler, ctx, manifest, natsclient.CFMOrchestrationSubject)

	require.NoError(t, err)

	var response model.OrchestrationResponse
	err = json.Unmarshal(capturedPayload, &response)
	require.NoError(t, err)

	assert.False(t, response.Success)
	assert.Regexp(t, `^orchestration could not be started \[[0-9a-f-]{36}\]$`, response.ErrorDetail)
	assert.NotContains(t, response.ErrorDetail, "provision failed")
	assert.Equal(t, "test-manifest-id", response.ManifestID)
	assert.Equal(t, testOrchestrationType, response.OrchestrationType)
	assert.NotEmpty(t, response.ID) // Should have a generated UUID
	assert.NotNil(t, response.Properties)
	assert.Equal(t, 0, len(response.Properties)) // Should be empty map

	mockProvisionManager.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestNatsProvisionHandler_Dispatcher_ErrorDetail(t *testing.T) {
	testCases := []struct {
		name     string
		error    error
		expected string
	}{
		{"ClientError", types.NewClientError("orchestration type 'x' not found"), "orchestration type 'x' not found"},
		{"wrapped ClientError", fmt.Errorf("transaction failed: %w", types.NewClientError("Missing required field: id")), "Missing required field: id"},
		{"FatalError", types.NewFatalWrappedError(errors.New("nats: key value store error"), "error performing de-duplication"), ""},
		{"StandardError", errors.New(`pq: relation "orchestration_definitions" does not exist`), ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := mocks.NewMockMsgClient(t)
			mockProvisionManager := &MockProvisionManager{}
			handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})

			ctx := context.Background()
			manifest := model.OrchestrationManifest{ID: "test-manifest-id", OrchestrationType: testOrchestrationType}
			mockProvisionManager.On("Start", ctx, inNamespace(manifest, model.DefaultNamespace)).Return(nil, tc.error)

			var capturedPayload []byte
			mockClient.EXPECT().Publish(ctx, natsclient.CFMOrchestrationResponseSubject, mock.Anything).
				Run(func(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) {
					capturedPayload = payload
				}).Return(&jetstream.PubAck{}, nil)

			require.NoError(t, dispatch(handler, ctx, manifest, natsclient.CFMOrchestrationSubject))

			var response model.OrchestrationResponse
			require.NoError(t, json.Unmarshal(capturedPayload, &response))
			if tc.expected != "" {
				assert.Equal(t, tc.expected, response.ErrorDetail)
			} else {
				assert.Regexp(t, `^orchestration could not be started \[[0-9a-f-]{36}\]$`, response.ErrorDetail)
			}
		})
	}
}

func TestNatsProvisionHandler_Dispatcher_ErrorTypes(t *testing.T) {
	testCases := []struct {
		name          string
		error         error
		shouldPublish bool
	}{
		{"RecoverableError", types.NewRecoverableError("network timeout"), false},
		{"ClientError", types.NewClientError("invalid manifest"), true},
		{"FatalError", types.NewFatalError("fatal processing error"), true},
		{"StandardError", errors.New("standard error"), true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			mockClient := mocks.NewMockMsgClient(t)
			mockProvisionManager := &MockProvisionManager{}

			handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})

			ctx := context.Background()
			manifest := model.OrchestrationManifest{
				ID:                "test-manifest-id",
				OrchestrationType: testOrchestrationType,
				Payload:           map[string]any{"key": "value"},
			}

			// Mock expectations
			mockProvisionManager.On("Start", ctx, inNamespace(manifest, model.DefaultNamespace)).Return(nil, tc.error)

			if tc.shouldPublish {
				pubAck := &jetstream.PubAck{}
				mockClient.EXPECT().Publish(ctx, natsclient.CFMOrchestrationResponseSubject, mock.Anything).Return(pubAck, nil)
			}

			err := dispatch(handler, ctx, manifest, natsclient.CFMOrchestrationSubject)

			if tc.shouldPublish {
				assert.NoError(t, err, "Non-recoverable errors should result in ACK")
			} else {
				assert.Error(t, err, "Recoverable errors should be returned")
				assert.True(t, types.IsRecoverable(err))
			}

			mockProvisionManager.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}

type MockProvisionManager struct {
	mock.Mock
}

func (m *MockProvisionManager) Start(ctx context.Context, manifest *model.OrchestrationManifest) (*api.Orchestration, error) {
	args := m.Called(ctx, manifest)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.Orchestration), args.Error(1)
}

func (m *MockProvisionManager) Cancel(ctx context.Context, namespace string, id string) error {
	args := m.Called(ctx, namespace, id)
	return args.Error(0)
}

func (m *MockProvisionManager) GetOrchestration(ctx context.Context, namespace string, id string) (*api.Orchestration, error) {
	args := m.Called(ctx, namespace, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.Orchestration), args.Error(1)
}

func (m *MockProvisionManager) QueryOrchestrations(ctx context.Context, namespace string, predicate query.Predicate, options store.PaginationOptions) iter.Seq2[*api.OrchestrationEntry, error] {
	panic("not implemented")
}

func (m *MockProvisionManager) CountOrchestrations(ctx context.Context, namespace string, predicate query.Predicate) (int64, error) {
	panic("not implemented")
}

func TestNatsProvisionHandler_Dispatcher_StartErrorIsPublishedToReplyTo(t *testing.T) {
	testCases := []struct {
		name     string
		replyTo  string
		expected string
	}{
		{"reply-to set", "second-manager-response", "event.default.second-manager-response"},
		{"no reply-to", "", natsclient.CFMOrchestrationResponseSubject},
		// An invalid token is rejected by Start; the rejection must still reach a subject in the stream
		{"invalid reply-to", "a.b", natsclient.CFMOrchestrationResponseSubject},
		{"reserved reply-to", natsclient.CFMOrchestration, natsclient.CFMOrchestrationResponseSubject},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := mocks.NewMockMsgClient(t)
			mockProvisionManager := &MockProvisionManager{}
			handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})

			ctx := context.Background()
			manifest := model.OrchestrationManifest{ID: "test-manifest-id", OrchestrationType: testOrchestrationType, ReplyTo: tc.replyTo}
			mockProvisionManager.On("Start", ctx, inNamespace(manifest, model.DefaultNamespace)).Return(nil, types.NewClientError("orchestration type 'x' not found"))

			var capturedPayload []byte
			mockClient.EXPECT().Publish(ctx, tc.expected, mock.Anything).
				Run(func(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) {
					capturedPayload = payload
				}).Return(&jetstream.PubAck{}, nil)

			require.NoError(t, dispatch(handler, ctx, manifest, natsclient.CFMOrchestrationSubject))

			var response model.OrchestrationResponse
			require.NoError(t, json.Unmarshal(capturedPayload, &response))
			assert.False(t, response.Success)
			assert.Equal(t, "test-manifest-id", response.ManifestID)
		})
	}
}

func TestNatsProvisionHandler_SubjectSelectsNamespace(t *testing.T) {
	for name, manifestNamespace := range map[string]string{"manifest without namespace": "", "manifest with namespace": "second"} {
		t.Run(name, func(t *testing.T) {
			mockClient := mocks.NewMockMsgClient(t)
			mockProvisionManager := &MockProvisionManager{}
			handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})
			ctx := context.Background()
			manifest := model.OrchestrationManifest{ID: "test-manifest-id", OrchestrationType: testOrchestrationType, Namespace: manifestNamespace}
			mockProvisionManager.On("Start", ctx, inNamespace(manifest, "second")).Return(&api.Orchestration{ID: "test-manifest-id"}, nil)

			require.NoError(t, dispatch(handler, ctx, manifest, natsclient.Subject("second", natsclient.CFMOrchestration)))

			mockProvisionManager.AssertExpectations(t)
		})
	}
}

func TestNatsProvisionHandler_RejectsManifestOfOtherNamespace(t *testing.T) {
	for name, tc := range map[string]struct {
		manifestNamespace string
		subjectNamespace  string
	}{
		"other namespace":   {"third", "second"},
		"default namespace": {model.DefaultNamespace, "second"},
		"from default":      {"second", model.DefaultNamespace},
	} {
		t.Run(name, func(t *testing.T) {
			mockClient := mocks.NewMockMsgClient(t)
			mockProvisionManager := &MockProvisionManager{}
			handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})
			ctx := context.Background()
			manifest := model.OrchestrationManifest{ID: "test-manifest-id", OrchestrationType: testOrchestrationType, Namespace: tc.manifestNamespace}

			// the rejection goes to the namespace of the sender, and the provision manager is not called
			var capturedPayload []byte
			mockClient.EXPECT().Publish(ctx, natsclient.ResponseSubject(tc.subjectNamespace, ""), mock.Anything).
				Run(func(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) {
					capturedPayload = payload
				}).Return(&jetstream.PubAck{}, nil)

			require.NoError(t, dispatch(handler, ctx, manifest, natsclient.Subject(tc.subjectNamespace, natsclient.CFMOrchestration)))

			var response model.OrchestrationResponse
			require.NoError(t, json.Unmarshal(capturedPayload, &response))
			assert.False(t, response.Success)
			assert.Contains(t, response.ErrorDetail, "is not the namespace")
			mockProvisionManager.AssertNotCalled(t, "Start", mock.Anything, mock.Anything)
		})
	}
}

func TestNatsProvisionHandler_DropsManifestOnUnknownSubject(t *testing.T) {
	mockClient := mocks.NewMockMsgClient(t)
	mockProvisionManager := &MockProvisionManager{}
	handler := newNatsProvisionHandler(mockClient, mockProvisionManager, system.NoopMonitor{})

	require.NoError(t, dispatch(handler, context.Background(), model.OrchestrationManifest{ID: "test-manifest-id"}, "event.cfm-orchestration"))

	mockProvisionManager.AssertNotCalled(t, "Start", mock.Anything, mock.Anything)
}
