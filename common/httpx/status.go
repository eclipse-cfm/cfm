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

package httpx

import (
	"fmt"
	"io"
	"net/http"
)

// maxErrorBodyBytes bounds how much of an error response body is kept in a StatusError.
const maxErrorBodyBytes = 4096

// StatusError is returned when a server answers with an unexpected HTTP status code. It classifies itself for
// common/types: a transient status (see IsTransientStatus) makes it a recoverable error, any other status a client error.
type StatusError struct {
	// Operation describes what was attempted, e.g. "failed to create Siglet Mapping".
	Operation  string
	StatusCode int
	// Body is the start of the response body, if the server sent one.
	Body string
}

// NewStatusError builds a StatusError from resp, keeping the start of the response body.
func NewStatusError(operation string, resp *http.Response) *StatusError {
	statusErr := &StatusError{Operation: operation, StatusCode: resp.StatusCode}
	if resp.Body != nil {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		statusErr.Body = string(body)
	}
	return statusErr
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s: received status code %d", e.Operation, e.StatusCode)
	}
	return fmt.Sprintf("%s: received status code %d, body: %s", e.Operation, e.StatusCode, e.Body)
}

// IsRecoverable reports whether retrying the request may succeed.
func (e *StatusError) IsRecoverable() bool { return IsTransientStatus(e.StatusCode) }

// IsClientError reports whether the request was rejected for good.
func (e *StatusError) IsClientError() bool { return !e.IsRecoverable() }

// IsTransientStatus reports whether an HTTP status code describes a condition that may clear up on its own: a server
// error (5xx), a request timeout (408) or rate limiting (429).
func IsTransientStatus(statusCode int) bool {
	return statusCode >= http.StatusInternalServerError ||
		statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests
}
