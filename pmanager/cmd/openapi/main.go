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

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/pmanager/model/v1alpha1"
	"github.com/oaswrap/spec"
	"github.com/oaswrap/spec/option"
)

const docsDir = "docs"

func main() {
	r := spec.NewRouter(
		option.WithTitle("Provision Manager API"),
		option.WithVersion("0.0.1"),
		option.WithDescription("API for managing Orchestrations, Orchestration Definitions, and Activity Definitions"),
		option.WithServer("http://localhost:8080", option.ServerDescription("Development server")),
	)

	generateOrchestrationEndpoints(r)
	generateOrchestrationDefinitionEndpoints(r)
	generateActivityDefinitionEndpoints(r)

	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		if err := os.Mkdir(docsDir, 0755); err != nil {
			panic(err)
		}
	}

	if err := r.WriteSchemaTo(filepath.Join(docsDir, "openapi.json")); err != nil {
		panic(err)
	}
}

func generateOrchestrationEndpoints(r spec.Generator) {
	orchestrations := r.Group("/api/v1alpha1/orchestrations", option.GroupTags("Orchestration"))

	orchestrations.Post("",
		option.Summary("Execute an Orchestration"),
		option.Description("Execute an Orchestration"),
		option.Request(model.OrchestrationManifest{}),
		option.Response(http.StatusAccepted, nil),
	)

	orchestrations.Post("query",
		option.Summary("Perform an Orchestration query"),
		option.Description("Perform an Orchestration query"),
		option.Request(model.Query{}),
		option.Response(http.StatusOK, []v1alpha1.OrchestrationEntry{}),
	)

	orchestrations.Get("/{id}",
		option.Summary("Get an Orchestration"),
		option.Description("Retrieve an Orchestration by ID"),
		option.Request(new(IDParam)),
		option.Response(http.StatusOK, v1alpha1.Orchestration{}),
	)
}

func generateActivityDefinitionEndpoints(r spec.Generator) {
	activity := r.Group("/api/v1alpha1/activity-definitions", option.GroupTags("ActivityDefinition"))

	activity.Get("",
		option.Summary("Get Activity Definitions"),
		option.Description("Returns all Activity Definitions"),
		option.Response(http.StatusOK, []v1alpha1.ActivityDefinitionDto{}),
	)

	activity.Post("",
		option.Summary("Create an Activity Definition"),
		option.Description("Create a new Activity Definition"),
		option.Request(v1alpha1.ActivityDefinitionDto{}),
		option.Response(http.StatusCreated, nil),
	)

	activity.Delete("/{type}",
		option.Summary("Delete an Activity Definition"),
		option.Description("Delete a new Activity Definition"),
		option.Request(new(TypeParam)),
		option.Response(http.StatusOK, nil))
}

func generateOrchestrationDefinitionEndpoints(r spec.Generator) {
	orchestration := r.Group("/api/v1alpha1/orchestration-definitions", option.GroupTags("OrchestrationDefinition"))

	orchestration.Get("",
		option.Summary("Get Orchestration Definitions"),
		option.Description("Returns all Orchestration Definitions"),
		option.Response(http.StatusOK, []v1alpha1.OrchestrationTemplate{}),
	)

	orchestration.Get("/{templateRef}",
		option.Request(new(TemplateRefParam)),
		option.Summary("Get all Orchestration Definitions by template ID"),
		option.Description("Get all Orchestration Definitions that were generated based on the given template ID"),
		option.Response(http.StatusOK, []v1alpha1.OrchestrationDefinitionDto{}),
		option.Response(http.StatusNotFound, nil, option.ContentDescription("No OrchestrationDefinition "+
			"with the given `templateRef` was found")))

	orchestration.Post("",
		option.Summary("Create Orchestration Definition(s) from an OrchestrationTemplate"),
		option.Description("Create Orchestration Definition(s) from an OrchestrationTemplate. The `id` field of "+
			"the response contains a unique identifier, by which the generated Orchestration Definitions are referenced."),
		option.Request(v1alpha1.OrchestrationTemplate{}),
		option.Response(http.StatusCreated, v1alpha1.IDResponse{}),
	)

	orchestration.Delete("/{templateRef}",
		option.Summary("Delete all Orchestration Definitions for the given template ID"),
		option.Description("Delete all Orchestration Definitions that were generated based on the given template ID"),
		option.Request(new(TemplateRefParam)),
		option.Response(http.StatusOK, nil),
		option.Response(http.StatusNotFound, nil, option.ContentDescription("No OrchestrationDefinition "+
			"with the given `templateRef` was found")))

}

type TypeParam struct {
	ID string `path:"type" required:"true"`
}
type TemplateRefParam struct {
	TemplateRef string `path:"templateRef" required:"true"`
}

type IDParam struct {
	ID string `path:"id" required:"true"`
}
