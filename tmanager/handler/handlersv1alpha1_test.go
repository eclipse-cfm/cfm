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
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/eclipse-cfm/cfm/tmanager/model/v1alpha1"
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
	queried    bool
	profile    *api.ParticipantProfile
}

func (f *fakeParticipantService) GetProfile(context.Context, string, string) (*api.ParticipantProfile, error) {
	return f.profile, nil
}

func (f *fakeParticipantService) QueryProfiles(context.Context, query.Predicate, store.PaginationOptions) iter.Seq2[*api.ParticipantProfile, error] {
	f.queried = true
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

type fakeTenantService struct {
	api.TenantService
	getErr error
}

func (f *fakeTenantService) GetTenant(_ context.Context, tenantID string) (*api.Tenant, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &api.Tenant{Entity: api.Entity{ID: tenantID}}, nil
}

func newTestHandler(participants api.ParticipantProfileService, dataspaces api.DataspaceProfileService) *TMHandler {
	return NewHandler(&fakeTenantService{}, participants, nil, dataspaces, nil, system.NoopMonitor{})
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

func TestGetProfilesForTenant_TenantNotAccessible(t *testing.T) {
	participants := &fakeParticipantService{}
	h := NewHandler(&fakeTenantService{getErr: types.ErrNotFound}, participants, nil, nil, nil, system.NoopMonitor{})
	w := newHeaderCountingRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tenants/t1/participant-profiles", nil)

	h.getProfilesForTenant(w, req, "t1")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, participants.queried)
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

func TestGetParticipantProfile_ErrorCauseRequiresAdminScope(t *testing.T) {
	profile := &api.ParticipantProfile{
		Entity:      api.Entity{ID: "p1"},
		TenantID:    "t1",
		Error:       true,
		ErrorDetail: "activity A1 of orchestration o1 failed",
		ErrorCause:  "unexpected status 500 from http://identityhub.internal",
	}
	h := newTestHandler(&fakeParticipantService{profile: profile}, nil)

	get := func(claims *cfmauth.Claims) v1alpha1.ParticipantProfile {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/tenants/t1/participant-profiles/p1", nil)
		newClaimsRouter(h, claims).ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var response v1alpha1.ParticipantProfile
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		return response
	}

	admin := get(&cfmauth.Claims{Subject: "operator", Scopes: []string{scopeTmRead, scopeTmAdmin}})
	assert.True(t, admin.Error)
	assert.Equal(t, profile.ErrorDetail, admin.ErrorDetail)
	assert.Equal(t, profile.ErrorCause, admin.ErrorCause)

	owner := get(&cfmauth.Claims{Subject: "client-1", Scopes: []string{scopeTmRead}})
	assert.True(t, owner.Error)
	assert.Equal(t, profile.ErrorDetail, owner.ErrorDetail, "the detail is written for the owner")
	assert.Empty(t, owner.ErrorCause)
}
