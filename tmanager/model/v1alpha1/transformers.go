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

package v1alpha1

import (
	"time"

	"github.com/google/uuid"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/tmanager/api"
)

func ToTenant(input *api.Tenant) *Tenant {
	return &Tenant{
		Entity: Entity{
			ID:      input.ID,
			Version: input.Version,
		},
		NewTenant: NewTenant{
			Properties: input.Properties,
		},
	}
}

func NewAPITenant(input *NewTenant) *api.Tenant {
	return &api.Tenant{
		Entity: api.Entity{
			ID:      uuid.New().String(),
			Version: 0,
		},
		Properties: api.ToProperties(input.Properties),
	}
}

func NewAPIDataspaceProfile(input *NewDataspaceProfile) *api.DataspaceProfile {
	cspecs := make([]model.CredentialSpec, len(input.DataspaceSpec.CredentialSpecs))
	for i, cspec := range input.DataspaceSpec.CredentialSpecs {
		cspecs[i] = model.CredentialSpec{
			Id:              cspec.Id,
			Type:            cspec.Type,
			Issuer:          cspec.Issuer,
			Format:          cspec.Format,
			ParticipantRole: cspec.ParticipantRole,
		}
	}
	return &api.DataspaceProfile{
		Entity: api.Entity{
			ID:      uuid.New().String(),
			Version: 0,
		},
		DataspaceSpec: api.DataspaceSpec{
			ProtocolStack:   input.DataspaceSpec.ProtocolStack,
			CredentialSpecs: cspecs,
		},
		Artifacts:   input.Artifacts,
		Deployments: make([]api.DataspaceDeployment, 0),
		Properties:  api.ToProperties(input.Properties),
	}
}

func ToParticipantProfile(input *api.ParticipantProfile) *ParticipantProfile {
	return &ParticipantProfile{
		Entity: Entity{
			ID:      input.ID,
			Version: input.Version,
		},
		Identifier:       input.Identifier,
		TenantID:         input.TenantID,
		ParticipantRoles: input.ParticipantRoles,
		VPAs:             ToVPACollection(input),
		Properties:       input.Properties,
		Error:            input.Error,
		ErrorDetail:      input.ErrorDetail,
	}
}

func ToVPACollection(input *api.ParticipantProfile) []VirtualParticipantAgent {
	vpas := make([]VirtualParticipantAgent, len(input.VPAs))
	for i, vpa := range input.VPAs {
		vpas[i] = *ToVPA(&vpa)
	}
	return vpas
}

func ToVPA(input *api.VirtualParticipantAgent) *VirtualParticipantAgent {
	return &VirtualParticipantAgent{
		DeployableEntity: DeployableEntity{
			Entity: Entity{
				ID:      input.ID,
				Version: input.Version,
			},
			State:          input.State.String(),
			StateTimestamp: input.StateTimestamp,
		},
		Type:       input.Type,
		CellID:     input.CellID,
		Properties: input.Properties,
	}
}

func ToCell(input *api.Cell) *Cell {
	return &Cell{
		Entity: Entity{
			ID:      input.ID,
			Version: input.Version,
		},
		NewCell: NewCell{
			State:          input.State.String(),
			StateTimestamp: input.StateTimestamp,
			Properties:     input.Properties,
			ExternalID:     input.ExternalID,
		},
	}
}

func ToAPIParticipantProfile(input *ParticipantProfile) *api.ParticipantProfile {
	return &api.ParticipantProfile{
		Entity: api.Entity{
			ID:      input.ID,
			Version: input.Version,
		},
		Identifier:       input.Identifier,
		TenantID:         input.TenantID,
		ParticipantRoles: input.ParticipantRoles,
		VPAs:             ToAPIVPACollection(input.VPAs),
		Properties:       api.ToProperties(input.Properties),
		Error:            input.Error,
		ErrorDetail:      input.ErrorDetail,
	}
}

func ToAPINewParticipantProfileDeployment(input *NewParticipantProfileDeployment) *api.NewParticipantProfileDeployment {
	dataSpaceProfileIDs := input.DataspaceProfileIDs
	if dataSpaceProfileIDs == nil {
		dataSpaceProfileIDs = make([]string, 0)
	}

	participantRoles := input.ParticipantRoles
	if participantRoles == nil {
		participantRoles = make(map[string][]string)
	}

	var vpaProperties api.VPAPropMap
	if input.VPAProperties == nil {
		vpaProperties = make(api.VPAPropMap)
	} else {
		vpaProperties = *api.ToVPAMap(input.VPAProperties)
	}

	properties := input.Properties
	if properties == nil {
		properties = make(map[string]any)
	}

	return &api.NewParticipantProfileDeployment{
		Identifier:          input.Identifier,
		CellID:              input.CellID,
		DataspaceProfileIDs: dataSpaceProfileIDs,
		ParticipantRoles:    participantRoles,
		VPAProperties:       vpaProperties,
		Properties:          properties,
	}
}

func ToAPIVPACollection(vpas []VirtualParticipantAgent) []api.VirtualParticipantAgent {
	apiVPAs := make([]api.VirtualParticipantAgent, len(vpas))
	for i, vpa := range vpas {
		apiVPAs[i] = *ToAPIVPA(&vpa)
	}
	return apiVPAs
}

func ToAPIVPA(input *VirtualParticipantAgent) *api.VirtualParticipantAgent {
	state, _ := api.ToDeploymentState(input.State)
	return &api.VirtualParticipantAgent{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      input.ID,
				Version: input.Version,
			},
			State:          state,
			StateTimestamp: input.StateTimestamp.UTC(), // Force UTC
		},
		Type:       input.Type,
		CellID:     input.CellID,
		Properties: api.ToProperties(input.Properties),
	}
}

func ToAPICell(input *Cell) *api.Cell {
	state, _ := api.ToDeploymentState(input.State)
	return &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      input.ID,
				Version: input.Version,
			},
			State:          state,
			StateTimestamp: input.StateTimestamp.UTC(), // Force UTC
		},
		Properties: api.ToProperties(input.Properties),
	}
}

func NewAPICell(input *NewCell) *api.Cell {
	state := api.DeploymentStateInitial
	if input.State != "" {
		if parsed, err := api.ToDeploymentState(input.State); err == nil {
			state = parsed
		}
	}

	stateTimestamp := input.StateTimestamp
	if stateTimestamp.IsZero() {
		stateTimestamp = time.Now()
	}

	return &api.Cell{
		DeployableEntity: api.DeployableEntity{
			Entity: api.Entity{
				ID:      uuid.New().String(),
				Version: 0,
			},
			State:          state,
			StateTimestamp: stateTimestamp.UTC(), // Force UTC
		},
		ExternalID: input.ExternalID,
		Properties: api.ToProperties(input.Properties),
	}
}

func ToDataspaceProfile(input *api.DataspaceProfile) *DataspaceProfile {
	deployments := make([]DataspaceDeployment, len(input.Deployments))
	for i, deployment := range input.Deployments {
		deployments[i] = DataspaceDeployment{
			DeployableEntity: DeployableEntity{
				Entity: Entity{
					ID:      deployment.ID,
					Version: deployment.Version,
				},
				State:          deployment.State.String(),
				StateTimestamp: deployment.StateTimestamp.UTC(), // Convert to UTC
			},
			CellID:         deployment.CellID,
			ExternalCellID: deployment.ExternalCellID,
			Properties:     deployment.Properties,
		}
	}

	cspecs := make([]CredentialSpec, len(input.DataspaceSpec.CredentialSpecs))
	for i, cspec := range input.DataspaceSpec.CredentialSpecs {
		cspecs[i] = CredentialSpec{
			Id:              cspec.Id,
			Type:            cspec.Type,
			Issuer:          cspec.Issuer,
			Format:          cspec.Format,
			ParticipantRole: cspec.ParticipantRole,
		}
	}

	return &DataspaceProfile{
		Entity: Entity{
			ID:      input.ID,
			Version: input.Version,
		},
		DataspaceSpec: DataspaceSpec{
			ProtocolStack:   input.DataspaceSpec.ProtocolStack,
			CredentialSpecs: cspecs,
		},
		Artifacts:   input.Artifacts,
		Deployments: deployments,
		Properties:  input.Properties,
	}
}
