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

package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockResponseWriter implements http.ResponseWriter for testing
type mockResponseWriter struct {
	headers    http.Header
	statusCode int
	body       *bytes.Buffer
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		headers: make(http.Header),
		body:    &bytes.Buffer{},
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.headers
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	return m.body.Write(data)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

func TestWriteErrorWithID(t *testing.T) {
	t.Run("writes error response with ID", func(t *testing.T) {
		w := newMockResponseWriter()
		handler := HttpHandler{Monitor: system.NoopMonitor{}}
		handler.WriteErrorWithID(w, "Test error", http.StatusBadRequest, "error-123")

		assert.Equal(t, http.StatusBadRequest, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response ErrorResponse
		err := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Bad Request", response.Error)
		assert.Equal(t, "Test error", response.Message)
		assert.Equal(t, 400, response.Code)
		assert.Equal(t, "error-123", response.ID)
	})

	t.Run("writes error response without ID", func(t *testing.T) {
		w := newMockResponseWriter()

		handler := HttpHandler{Monitor: system.NoopMonitor{}}
		handler.WriteErrorWithID(w, "Server error", http.StatusInternalServerError, "")

		assert.Equal(t, http.StatusInternalServerError, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response ErrorResponse
		err := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Equal(t, "Server error", response.Message)
		assert.Equal(t, 500, response.Code)
		assert.Empty(t, response.ID)
	})

	t.Run("handles different status codes", func(t *testing.T) {
		testCases := []struct {
			statusCode    int
			expectedError string
		}{
			{http.StatusNotFound, "Not Found"},
			{http.StatusUnauthorized, "Unauthorized"},
			{http.StatusForbidden, "Forbidden"},
		}

		handler := HttpHandler{Monitor: system.NoopMonitor{}}

		for _, tc := range testCases {
			w := newMockResponseWriter()
			handler.WriteErrorWithID(w, "Test message", tc.statusCode, "test-id")

			assert.Equal(t, tc.statusCode, w.statusCode)

			var response ErrorResponse
			err := json.Unmarshal(w.body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedError, response.Error)
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Run("delegates to WriteErrorWithID with empty ID", func(t *testing.T) {
		w := newMockResponseWriter()
		handler := HttpHandler{Monitor: system.NoopMonitor{}}

		handler.WriteError(w, "Test message", http.StatusInternalServerError)

		assert.Equal(t, http.StatusInternalServerError, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response ErrorResponse
		err := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Internal Server Error", response.Error)
		assert.Equal(t, "Test message", response.Message)
		assert.Equal(t, 500, response.Code)
		assert.Empty(t, response.ID)
	})
}

func TestInvalidMethod(t *testing.T) {
	t.Run("returns false when method matches", func(t *testing.T) {
		w := newMockResponseWriter()
		req, err := http.NewRequest(http.MethodPost, "/test", nil)
		require.NoError(t, err)

		handler := HttpHandler{Monitor: system.NoopMonitor{}}
		result := handler.InvalidMethod(w, req, http.MethodPost)

		assert.False(t, result)
		assert.Equal(t, 0, w.statusCode)
	})

	t.Run("returns true and writes error when method doesn't match", func(t *testing.T) {
		w := newMockResponseWriter()
		req, err := http.NewRequest(http.MethodGet, "/test", nil)
		require.NoError(t, err)

		handler := HttpHandler{Monitor: system.NoopMonitor{}}
		result := handler.InvalidMethod(w, req, http.MethodPost)

		assert.True(t, result)
		assert.Equal(t, http.StatusMethodNotAllowed, w.statusCode)

		var response ErrorResponse
		err = json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Method not allowed", response.Message)
		assert.Equal(t, 405, response.Code)
	})
}

func TestHandleError(t *testing.T) {
	handler := HttpHandler{Monitor: system.NoopMonitor{}}

	t.Run("handles ErrNotFound", func(t *testing.T) {
		w := newMockResponseWriter()
		err := types.ErrNotFound

		handler.HandleError(w, err)

		assert.Equal(t, http.StatusNotFound, w.statusCode)

		var response ErrorResponse
		jsonErr := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, jsonErr)

		assert.NotEmpty(t, response.Message)
		assert.Equal(t, 404, response.Code)
	})

	t.Run("handles BadRequestError", func(t *testing.T) {
		w := newMockResponseWriter()
		err := &types.BadRequestError{Message: "invalid input"}

		handler.HandleError(w, err)

		assert.Equal(t, http.StatusBadRequest, w.statusCode)

		var response ErrorResponse
		jsonErr := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, jsonErr)

		assert.NotEmpty(t, response.Message)
		assert.Equal(t, 400, response.Code)
	})

	t.Run("handles SystemError", func(t *testing.T) {
		w := newMockResponseWriter()
		err := &types.SystemError{Message: "database connection failed"}

		handler.HandleError(w, err)

		assert.Equal(t, http.StatusInternalServerError, w.statusCode)

		var response ErrorResponse
		jsonErr := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, jsonErr)

		assert.Contains(t, response.Message, "Internal server error occurred [")
		assert.Equal(t, 500, response.Code)
	})

	t.Run("handles FatalError", func(t *testing.T) {
		w := newMockResponseWriter()
		err := types.SystemError{Message: "fatal system error"}

		handler.HandleError(w, err)

		assert.Equal(t, http.StatusInternalServerError, w.statusCode)

		var response ErrorResponse
		jsonErr := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, jsonErr)

		assert.Contains(t, response.Message, "Internal server error occurred [") // Do not report internal error details
		assert.Equal(t, 500, response.Code)
	})

	t.Run("handles generic error", func(t *testing.T) {
		w := newMockResponseWriter()
		err := errors.New("generic error message")

		handler.HandleError(w, err)

		assert.Equal(t, http.StatusInternalServerError, w.statusCode)

		var response ErrorResponse
		jsonErr := json.Unmarshal(w.body.Bytes(), &response)
		require.NoError(t, jsonErr)

		assert.NotEmpty(t, response.Message)
		assert.Equal(t, 500, response.Code)
	})
}

func TestCreated(t *testing.T) {
	t.Run("sets correct status code and content type", func(t *testing.T) {
		w := newMockResponseWriter()
		handler := HttpHandler{Monitor: system.NoopMonitor{}}

		handler.Created(w)

		assert.Equal(t, http.StatusCreated, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Empty(t, w.body.String())
	})
}

func TestAccepted(t *testing.T) {
	t.Run("sets correct status code and content type", func(t *testing.T) {
		w := newMockResponseWriter()
		handler := HttpHandler{Monitor: system.NoopMonitor{}}

		handler.Accepted(w)

		assert.Equal(t, http.StatusAccepted, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Empty(t, w.body.String())
	})
}

func TestResponseAccepted(t *testing.T) {
	t.Run("sets accepted status and writes response", func(t *testing.T) {
		w := newMockResponseWriter()
		handler := HttpHandler{Monitor: system.NoopMonitor{}}
		response := map[string]string{"message": "accepted"}

		handler.ResponseAccepted(w, response)

		assert.Equal(t, http.StatusAccepted, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var result map[string]string
		err := json.Unmarshal(w.body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "accepted", result["message"])
	})
}

func TestOK(t *testing.T) {
	t.Run("sets correct status code and content type", func(t *testing.T) {
		w := newMockResponseWriter()
		handler := HttpHandler{Monitor: system.NoopMonitor{}}

		handler.OK(w)

		assert.Equal(t, http.StatusOK, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Empty(t, w.body.String())
	})
}

func TestResponseOK(t *testing.T) {
	t.Run("sets OK status and writes response", func(t *testing.T) {
		w := newMockResponseWriter()
		handler := HttpHandler{Monitor: system.NoopMonitor{}}
		response := map[string]string{"status": "ok"}

		handler.ResponseOK(w, response)

		assert.Equal(t, http.StatusOK, w.statusCode)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var result map[string]string
		err := json.Unmarshal(w.body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "ok", result["status"])
	})
}
