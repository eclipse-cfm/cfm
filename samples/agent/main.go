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

// The sample agent executes the activities of the type "sample-storage". It shows how an activity agent is built on the
// agent framework in pmanager/natsagent, without any reference to dataspaces. See samples/README.md.
package main

import (
	"fmt"

	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

const (
	agentName = "Sample Storage Agent"
	// configPrefix is the name of the configuration file and the prefix of the environment variables of the agent, for
	// example SAMPLEAGENT_URI
	configPrefix = "sampleagent"
	// activityType is the type of the activity definition the agent executes the activities of
	activityType = "sample-storage"

	// The orchestration types of samples/definitions/orchestration-template.json. The discriminator of an activity is
	// the type of the orchestration it runs in.
	deployType  = "sample.workspace.deploy"
	disposeType = "sample.workspace.dispose"

	bucketNameKey = "bucketName"
)

func main() {
	natsagent.LaunchAgent(runtime.CreateSignalShutdownChan(), natsagent.LauncherConfig{
		AgentName:    agentName,
		ServiceName:  "cfm.sample.storage-agent",
		ConfigPrefix: configPrefix,
		ActivityType: activityType,
		NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
			return &storageProcessor{monitor: ctx.Monitor}
		},
	})
}

// workspaceInput are the fields of the manifest payload that the agent needs. They are declared in the input schema of
// the activity definition, see samples/definitions/activity-definition.json.
type workspaceInput struct {
	Workspace string `json:"workspace" validate:"required"`
	Region    string `json:"region"`
}

type storageProcessor struct {
	monitor system.LogMonitor
}

// Process is called for every activity of the type "sample-storage". An activity may be delivered more than once, so it
// must be idempotent.
func (p *storageProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	var input workspaceInput
	if err := ctx.ReadValues(&input); err != nil {
		// the orchestration fails, and is compensated if its definition names a compensation type
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("invalid input: %w", err)}
	}
	bucketName := "bucket-" + input.Workspace

	switch ctx.Discriminator() {
	case deployType:
		p.monitor.Infof("Creating storage %s in region %s for orchestration %s", bucketName, input.Region, ctx.OID())
		// a real agent would create the storage here

		// available to the activities that depend on this one
		ctx.SetValue(bucketNameKey, bucketName)
		// returned to the manager in the properties of the orchestration response
		ctx.SetOutputValue(bucketNameKey, bucketName)
		return api.ActivityResult{Result: api.ActivityResultComplete}
	case disposeType:
		p.monitor.Infof("Deleting storage %s for orchestration %s", bucketName, ctx.OID())
		// a real agent would delete the storage here, and succeed if it does not exist
		return api.ActivityResult{Result: api.ActivityResultComplete}
	default:
		return api.ActivityResult{
			Result: api.ActivityResultFatalError,
			Error:  fmt.Errorf("unsupported orchestration type %s", ctx.Discriminator()),
		}
	}
}
