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

package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/tmanager/model/v1alpha1"
	"github.com/oaswrap/spec"
	"github.com/oaswrap/spec/option"
)

const docsDir = "docs"

func main() {
	r := spec.NewRouter(
		option.WithTitle("Tenant Manager API"),
		option.WithVersion("0.0.1"),
		option.WithDescription("API for managing Tenants, Participant Profiles, Cells, and Dataspace Profiles"),
		option.WithServer("http://localhost:8080", option.ServerDescription("Development server")),
	)

	generateTenantEndpoints(r)
	generateParticipantEndpoints(r)
	generateParticipantQueryEndpoint(r)
	generateCellEndpoints(r)
	generateDataspaceEndpoints(r)

	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		if err := os.Mkdir(docsDir, 0755); err != nil {
			panic(err)
		}
	}

	if err := r.WriteSchemaTo(filepath.Join("docs", "openapi.json")); err != nil {
		panic(err)
	}
}

func generateParticipantQueryEndpoint(r spec.Generator) {
	participants := r.Group("/api/v1alpha1/participant-profiles")
	participants.Post("query",
		option.Summary("Perform a Participant Profile query"),
		option.Description("Perform a Participant Profile query"),
		option.Request(model.Query{}),
		option.Response(http.StatusOK, []v1alpha1.ParticipantProfile{}))
}

func generateTenantEndpoints(r spec.Generator) {
	tenants := r.Group("/api/v1alpha1/tenants")
	tenants.Get("",
		option.Summary("List Tenants"),
		option.Description("Retrieve all Tenants"),
		option.Response(http.StatusOK, []v1alpha1.Tenant{}),
	)

	tenants.Post("",
		option.Summary("Create a new Tenant"),
		option.Description("Create a new Tenant"),
		option.Request(v1alpha1.NewTenant{}),
		option.Response(http.StatusCreated, v1alpha1.Tenant{}),
	)

	tenants.Post("query",
		option.Summary("Perform a Tenant query"),
		option.Description("Perform a Tenant query"),
		option.Request(model.Query{}),
		option.Response(http.StatusOK, []v1alpha1.Tenant{}),
	)

	tenants.Get("/{id}",
		option.Summary("Get Tenant"),
		option.Description("Retrieve a Tenant by ID"),
		option.Request(new(IDParam)),
		option.Response(http.StatusOK, v1alpha1.Tenant{}),
	)

	tenants.Delete("/{id}",
		option.Summary("Delete Tenant"),
		option.Description("Deletes a Tenant by ID"),
		option.Request(new(IDParam)),
		option.Response(http.StatusOK, nil),
	)

	tenants.Patch("/{id}",
		option.Summary("Updates a Tenant"),
		option.Description("Updates a Tenant by ID"),
		option.Request(new(IDParam)),
		option.Request(v1alpha1.TenantPropertiesDiff{}),
		option.Response(http.StatusOK, v1alpha1.Tenant{}),
	)
}

func generateParticipantEndpoints(r spec.Generator) {
	participants := r.Group("/api/v1alpha1/tenants/{id}/participant-profiles")

	participants.Get("",
		option.Summary("List Participant Profiles"),
		option.Description("Retrieve all Participant Profiles"),
		option.Request(new(IDParam)),
		option.Response(http.StatusOK, []v1alpha1.ParticipantProfile{}),
	)

	participants.Post("",
		option.Summary("Create Participant Profile"),
		option.Description("Create a new Participant Profile"),
		option.Request(new(IDParam)),
		option.Request(v1alpha1.ParticipantProfile{}),
		option.Response(http.StatusCreated, v1alpha1.ParticipantProfile{}),
	)

	participants.Get("/{participantID}",
		option.Summary("Get Participant Profile"),
		option.Description("Get a Participant Profile"),
		option.Request(new(IDParam)),
		option.Request(new(ParticipantIDParam)),
		option.Response(http.StatusOK, v1alpha1.ParticipantProfile{}),
	)
	participants.Delete("/{participantID}",
		option.Summary("Dispose Participant Profile"),
		option.Description("Dispose a Participant Profile"),
		option.Request(new(IDParam)),
		option.Request(new(ParticipantIDParam)),
		option.Request(v1alpha1.ParticipantProfile{}),
		option.Response(http.StatusOK, v1alpha1.ParticipantProfile{}),
	)
}

func generateCellEndpoints(r spec.Generator) {
	cells := r.Group("/api/v1alpha1/cells")

	cells.Get("",
		option.Summary("List Cells"),
		option.Description("Retrieve all Cells"),
		option.Response(http.StatusOK, []v1alpha1.Cell{}),
	)

	cells.Post("",
		option.Summary("Create Cell"),
		option.Description("Create a new Cell"),
		option.Request(v1alpha1.NewCell{}),
		option.Response(http.StatusCreated, v1alpha1.Cell{}),
	)

	cells.Delete("/{id}",
		option.Summary("Delete Cell"),
		option.Description("Deletes a Cell by ID"),
		option.Request(new(IDParam)),
		option.Response(http.StatusOK, nil),
	)

}

func generateDataspaceEndpoints(r spec.Generator) {
	dataspaces := r.Group("/api/v1alpha1/dataspace-profiles")

	dataspaces.Get("",
		option.Summary("List Dataspace Profiles"),
		option.Description("Retrieve all dataspace profiles"),
		option.Response(http.StatusOK, []v1alpha1.DataspaceProfile{}),
	)

	dataspaces.Post("",
		option.Summary("Create Dataspace Profile"),
		option.Description("Create a new Dataspace Profile"),
		option.Request(v1alpha1.NewDataspaceProfile{}),
		option.Response(http.StatusCreated, v1alpha1.DataspaceProfile{}),
	)

	dataspaces.Get("/{id}",
		option.Summary("Get Dataspace Profile"),
		option.Description("Retrieve a Dataspace Profile by ID"),
		option.Request(new(IDParam)),
		option.Response(http.StatusOK, v1alpha1.DataspaceProfile{}),
	)

	dataspaces.Delete("/{id}",
		option.Summary("Delete Dataspace Profile"),
		option.Description("Deletes a Dataspace Profile by ID"),
		option.Request(new(IDParam)),
		option.Response(http.StatusOK, nil),
	)

	dataspaces.Post("/{id}/deployments",
		option.Summary("Deploy a Dataspace Profile"),
		option.Description("Deploy a Dataspace Profile"),
		option.Request(new(IDParam)),
		option.Response(http.StatusAccepted, nil),
	)
}

type IDParam struct {
	ID string `path:"id" required:"true"`
}

type ParticipantIDParam struct {
	ID string `path:"participantID" required:"true"`
}
