/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package launcher

import (
	"github.com/eclipse-cfm/cfm/agent/lifecycle/contractdefinitionagent/handler"
	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
)

const ContractDefinitionCreatedSubject = "events.contract.definition.created"

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := lifecycleagent.LauncherConfig[handler.ContractDefinitionEvent]{
		AgentName:    "Contract Definition Agent",
		ServiceName:  "cfm.agent.contractdefinition",
		ConfigPrefix: "cdagent",
		Subjects:     []string{"events.contract.definition.>"},
		NewProcessor: func(ctx *lifecycleagent.AgentContext) lifecycleagent.EventProcessor[handler.ContractDefinitionEvent] {
			return handler.NewProcessor(&handler.Config{
				LogMonitor: ctx.Monitor,
			})
		},
	}
	lifecycleagent.LaunchAgent(shutdown, config)
}
