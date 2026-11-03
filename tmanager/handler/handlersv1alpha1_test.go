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
	"context"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headerCountingRecorder records how often WriteHeader is called so tests can detect a second response being written.
type headerCountingRecorder struct {
	*httptest.ResponseRecorder
	writeHeaderCalls int
}

func newHeaderCountingRecorder() *headerCountingRecorder {
	return &headerCountingRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (r *headerCountingRecorder) WriteHeader(code int) {
	r.writeHeaderCalls++
	r.ResponseRecorder.WriteHeader(code)
}

type fakeParticipantService struct {
	api.ParticipantProfileService
	queryErr   error
	disposeErr error
}

func (f *fakeParticipantService) QueryProfiles(context.Context, query.Predicate, store.PaginationOptions) iter.Seq2[*api.ParticipantProfile, error] {
	return func(yield func(*api.ParticipantProfile, error) bool) {
		yield(nil, f.queryErr)
	}
}

func (f *fakeParticipantService) DisposeProfile(context.Context, string, string) error {
	return f.disposeErr
}

type fakeDataspaceService struct {
	api.DataspaceProfileService
	deployedProfileID string
	deployedCellID    string
}

func (f *fakeDataspaceService) DeployProfile(_ context.Context, profileID string, cellID string) error {
	f.deployedProfileID = profileID
	f.deployedCellID = cellID
	return nil
}

func newTestHandler(participants api.ParticipantProfileService, dataspaces api.DataspaceProfileService) *TMHandler {
	return NewHandler(nil, participants, nil, dataspaces, nil, system.NoopMonitor{})
}

func TestGetProfilesForTenant_QueryError(t *testing.T) {
	h := newTestHandler(&fakeParticipantService{queryErr: types.ErrNotFound}, nil)
	w := newHeaderCountingRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tenants/t1/participant-profiles", nil)

	require.NotPanics(t, func() {
		h.getProfilesForTenant(w, req, "t1")
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, 1, w.writeHeaderCalls)
}

func TestDisposeParticipantProfile_Error(t *testing.T) {
	h := newTestHandler(&fakeParticipantService{disposeErr: types.ErrNotFound}, nil)
	w := newHeaderCountingRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/tenants/t1/participant-profiles/p1", nil)

	h.disposeParticipantProfile(w, req, "t1", "p1")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, 1, w.writeHeaderCalls)
}

func TestDisposeParticipantProfile_Success(t *testing.T) {
	h := newTestHandler(&fakeParticipantService{}, nil)
	w := newHeaderCountingRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/tenants/t1/participant-profiles/p1", nil)

	h.disposeParticipantProfile(w, req, "t1", "p1")

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, 1, w.writeHeaderCalls)
}

func TestDeployDataspaceProfile(t *testing.T) {
	t.Run("deploys the profile named in the path", func(t *testing.T) {
		dataspaces := &fakeDataspaceService{}
		h := newTestHandler(nil, dataspaces)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/dataspace-profiles/dp1/deployments",
			strings.NewReader(`{"profileId":"dp1","cellId":"cell1"}`))

		h.deployDataspaceProfile(w, req, "dp1")

		assert.Equal(t, http.StatusAccepted, w.Code)
		assert.Equal(t, "dp1", dataspaces.deployedProfileID)
		assert.Equal(t, "cell1", dataspaces.deployedCellID)
	})

	t.Run("rejects a body profileId that differs from the path", func(t *testing.T) {
		dataspaces := &fakeDataspaceService{}
		h := newTestHandler(nil, dataspaces)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/dataspace-profiles/dp1/deployments",
			strings.NewReader(`{"profileId":"other","cellId":"cell1"}`))

		h.deployDataspaceProfile(w, req, "dp1")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, dataspaces.deployedProfileID)
	})
}
