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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/stretchr/testify/assert"
)

func TestStatusError_Classification(t *testing.T) {
	for status, recoverable := range map[int]bool{
		http.StatusInternalServerError: true,
		http.StatusBadGateway:          true,
		http.StatusServiceUnavailable:  true,
		http.StatusRequestTimeout:      true,
		http.StatusTooManyRequests:     true,
		http.StatusBadRequest:          false,
		http.StatusUnauthorized:        false,
		http.StatusNotFound:            false,
		http.StatusConflict:            false,
	} {
		err := error(&StatusError{Operation: "op", StatusCode: status})
		assert.Equal(t, recoverable, types.IsRecoverable(err), "status %d", status)
		assert.Equal(t, !recoverable, types.IsClientError(err), "status %d", status)
	}
}

func TestNewStatusError_KeepsBody(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("bad mapping"))}

	err := NewStatusError("failed to create Siglet Mapping", resp)

	assert.EqualError(t, err, "failed to create Siglet Mapping: received status code 400, body: bad mapping")
}

func TestNewStatusError_LimitsBody(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", 10*maxErrorBodyBytes)))}

	assert.Equal(t, strings.Repeat("x", maxErrorBodyBytes)+" (truncated)", NewStatusError("op", resp).Body)
}

func TestReadErrorBody(t *testing.T) {
	read := func(body string) string {
		return ReadErrorBody(&http.Response{Body: io.NopCloser(strings.NewReader(body))})
	}

	assert.Equal(t, "", ReadErrorBody(nil))
	assert.Equal(t, "", ReadErrorBody(&http.Response{}))
	assert.Equal(t, "", read(""))
	assert.Equal(t, `{"error": "bad"}`, read(`{"error": "bad"}`))
	assert.Equal(t, "line one line two", read("line one\r\nline two\n"), "line breaks must not reach log lines")
	assert.Equal(t, "a b", read("a\x00\x1b\tb"))
	assert.Equal(t, "ok", read("o\xffk"), "invalid UTF-8 is dropped")
	assert.Equal(t, strings.Repeat("x", maxErrorBodyBytes), read(strings.Repeat("x", maxErrorBodyBytes)), "a body at the limit is not truncated")

	// A multibyte character split by the limit is dropped rather than kept as an invalid byte
	split := read(strings.Repeat("x", maxErrorBodyBytes-1) + "é")
	assert.Equal(t, strings.Repeat("x", maxErrorBodyBytes-1)+" (truncated)", split)
}
