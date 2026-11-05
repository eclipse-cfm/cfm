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

import "github.com/eclipse-cfm/cfm/tmanager/api"

// participantUsesCell reports whether the participant has a VPA in the cell that isn't disposed.
func participantUsesCell(profile *api.ParticipantProfile, cellID string) bool {
	for _, vpa := range profile.VPAs {
		if vpa.CellID == cellID && vpa.State != api.DeploymentStateDisposed {
			return true
		}
	}
	return false
}

// participantIsDisposed reports whether all VPAs of the participant are disposed. A participant without VPAs isn't.
func participantIsDisposed(profile *api.ParticipantProfile) bool {
	if len(profile.VPAs) == 0 {
		return false
	}
	for _, vpa := range profile.VPAs {
		if vpa.State != api.DeploymentStateDisposed {
			return false
		}
	}
	return true
}

// dataspaceProfileUsesCell reports whether the dataspace profile has a deployment in the cell that isn't disposed.
func dataspaceProfileUsesCell(profile *api.DataspaceProfile, cellID string) bool {
	for _, deployment := range profile.Deployments {
		if deployment.CellID == cellID && deployment.State != api.DeploymentStateDisposed {
			return true
		}
	}
	return false
}
