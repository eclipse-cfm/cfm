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
	"strings"
	"unicode"
)

// maxErrorBodyBytes bounds how much of an error response body is kept in an error. Error messages end up in logs and in
// persisted orchestration state, so only the start of a body is kept.
const maxErrorBodyBytes = 1024

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
	return &StatusError{Operation: operation, StatusCode: resp.StatusCode, Body: ReadErrorBody(resp)}
}

// ReadErrorBody returns the start of an error response's body as a single line that is safe to put into an error
// message: at most 1 KiB is read, invalid UTF-8 is dropped, control characters such as line breaks are replaced by spaces, and a cut-off
// body is marked as truncated. It returns "" if there is no body.
func ReadErrorBody(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes+1))
	truncated := len(raw) > maxErrorBodyBytes
	if truncated {
		raw = raw[:maxErrorBodyBytes]
	}
	// Dropping invalid UTF-8 also drops a multibyte character split by the cut.
	body := strings.ToValidUTF8(string(raw), "")
	body = strings.Join(strings.FieldsFunc(body, unicode.IsControl), " ")
	body = strings.TrimSpace(body)
	if truncated && body != "" {
		body += " (truncated)"
	}
	return body
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
