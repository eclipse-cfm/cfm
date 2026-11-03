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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func TestOrchestrationRoutes_NoPostOnOrchestrationID(t *testing.T) {
	router := chi.NewRouter()
	assembly := &HandlerServiceAssembly{}
	assembly.registerOrchestrationRoutes(router, NewHandler(nil, nil, nil, system.NoopMonitor{}))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/orchestrations/template-1", nil))

	// The route used to dispatch to the orchestration-definition delete handler. It must not be routed at all now:
	// the router itself rejects the method, so no handler writes a response body.
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Empty(t, w.Body.String())
}
