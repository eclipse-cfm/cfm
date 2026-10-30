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

	"github.com/stretchr/testify/require"
)

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestDrainAndClose(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader("some content")}

	DrainAndClose(&http.Response{Body: body})

	require.True(t, body.closed)
	remaining, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Empty(t, remaining)
}

func TestDrainAndClose_NilResponse(t *testing.T) {
	require.NotPanics(t, func() {
		DrainAndClose(nil)
		DrainAndClose(&http.Response{})
	})
}
