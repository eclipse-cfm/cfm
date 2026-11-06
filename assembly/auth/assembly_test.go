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

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopValidator_AttachesAnonymousClaims(t *testing.T) {
	var got *cfmauth.Claims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = cfmauth.ClaimsFromContext(r.Context())
	})

	(&noopValidator{}).Middleware()(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	require.NotNil(t, got)
	assert.Equal(t, cfmauth.AnonymousSubject, got.Subject)
	assert.True(t, got.HasScope("tenant-manager-api:write"))
}
