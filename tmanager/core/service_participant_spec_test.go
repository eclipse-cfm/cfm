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

package core

import (
	"testing"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/tmanager/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCredentialSpecs(t *testing.T) {
	t.Run("returns empty list when no dataspace profiles provided", func(t *testing.T) {
		participantRoles := map[string][]string{
			"role1": {"permission1"},
		}
		dProfiles := []api.DataspaceProfile{}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 0, len(result))
	})

	t.Run("returns empty list when profiles have no credential specs", func(t *testing.T) {
		participantRoles := map[string][]string{
			"role1": {"permission1"},
		}
		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{},
				},
			},
		}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 0, len(result))
	})

	t.Run("includes specs with empty participant role regardless of participant roles", func(t *testing.T) {
		participantRoles := map[string][]string{}
		spec := model.CredentialSpec{
			Type:            "oauth2",
			Issuer:          "https://issuer.example.com",
			Format:          "jwt",
			ParticipantRole: "", // Empty role
		}
		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{spec},
				},
			},
		}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, spec, result[0])
	})

	t.Run("includes specs matching participant roles", func(t *testing.T) {
		participantRoles := map[string][]string{
			"ds-1": {"provider"},
		}
		spec := model.CredentialSpec{
			Type:            "oauth2",
			Issuer:          "https://issuer.example.com",
			Format:          "jwt",
			ParticipantRole: "provider",
		}
		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{spec},
				},
			},
		}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, spec, result[0])
	})

	t.Run("excludes specs with non-matching participant roles", func(t *testing.T) {
		participantRoles := map[string][]string{
			"ds-1": {"provider", "manufacturer"},
		}
		matchingSpec := model.CredentialSpec{
			Type:            "oauth2",
			Issuer:          "https://issuer.example.com",
			Format:          "jwt",
			ParticipantRole: "provider",
		}
		nonMatchingSpec := model.CredentialSpec{
			Type:            "saml",
			Issuer:          "https://issuer2.example.com",
			Format:          "saml2",
			ParticipantRole: "consumer",
		}
		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{matchingSpec, nonMatchingSpec},
				},
			},
		}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, matchingSpec, result[0])
	})

	t.Run("includes specs from multiple dataspace profiles", func(t *testing.T) {
		participantRoles := map[string][]string{
			"ds-1": {"role1"},
		}
		spec1 := model.CredentialSpec{
			Type:            "oauth2",
			Issuer:          "https://issuer1.example.com",
			Format:          "jwt",
			ParticipantRole: "role1",
		}
		spec2 := model.CredentialSpec{
			Type:            "saml",
			Issuer:          "https://issuer2.example.com",
			Format:          "saml2",
			ParticipantRole: "",
		}
		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{spec1},
				},
			},
			{
				Entity: api.Entity{ID: "ds-2", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{spec2},
				},
			},
		}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, spec1, result[0])
		assert.Equal(t, spec2, result[1])
	})

	t.Run("handles complex filtering across multiple profiles and roles", func(t *testing.T) {
		participantRoles := map[string][]string{
			"ds-1": {"provider", "validator"},
			"ds-2": {"unknown", "validator"},
		}
		// Specs that should be included
		providerSpec := model.CredentialSpec{
			Type:            "oauth2",
			Issuer:          "https://issuer1.example.com",
			Format:          "jwt",
			ParticipantRole: "provider",
		}
		globalSpec := model.CredentialSpec{
			Type:            "mtls",
			Issuer:          "https://ca.example.com",
			Format:          "pem",
			ParticipantRole: "", // Global spec
		}
		validatorSpec := model.CredentialSpec{
			Type:            "saml",
			Issuer:          "https://issuer2.example.com",
			Format:          "saml2",
			ParticipantRole: "validator",
		}
		// Specs that should be excluded
		unknownRoleSpec := model.CredentialSpec{
			Type:            "custom",
			Issuer:          "https://issuer3.example.com",
			Format:          "custom",
			ParticipantRole: "unknown",
		}

		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{providerSpec, globalSpec, unknownRoleSpec},
				},
			},
			{
				Entity: api.Entity{ID: "ds-2", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{validatorSpec},
				},
			},
		}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 3, len(result))

		// Check that included specs are present
		specs := map[string]bool{}
		for _, spec := range result {
			specs[spec.Type] = true
		}
		assert.True(t, specs["oauth2"])
		assert.True(t, specs["mtls"])
		assert.True(t, specs["saml"])
		assert.False(t, specs["custom"]) // Should not be included
	})

	t.Run("handles nil participant roles map", func(t *testing.T) {
		spec := model.CredentialSpec{
			Type:            "oauth2",
			Issuer:          "https://issuer.example.com",
			Format:          "jwt",
			ParticipantRole: "", // Empty role
		}
		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{spec},
				},
			},
		}

		result := generateCredentialSpecs(nil, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, spec, result[0])
	})

	t.Run("handles mixed empty and non-empty role specs", func(t *testing.T) {
		participantRoles := map[string][]string{
			"ds-1": {"provider"},
		}
		emptyRoleSpec := model.CredentialSpec{
			Type:            "mtls",
			Issuer:          "https://ca.example.com",
			Format:          "pem",
			ParticipantRole: "",
		}
		matchingRoleSpec := model.CredentialSpec{
			Type:            "oauth2",
			Issuer:          "https://issuer.example.com",
			Format:          "jwt",
			ParticipantRole: "provider",
		}
		nonMatchingRoleSpec := model.CredentialSpec{
			Type:            "saml",
			Issuer:          "https://issuer2.example.com",
			Format:          "saml2",
			ParticipantRole: "consumer",
		}
		dProfiles := []api.DataspaceProfile{
			{
				Entity: api.Entity{ID: "ds-1", Version: 1},
				DataspaceSpec: api.DataspaceSpec{
					CredentialSpecs: []model.CredentialSpec{emptyRoleSpec, matchingRoleSpec, nonMatchingRoleSpec},
				},
			},
		}

		result := generateCredentialSpecs(participantRoles, dProfiles)

		require.NotNil(t, result)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, emptyRoleSpec, result[0])
		assert.Equal(t, matchingRoleSpec, result[1])
	})
}
