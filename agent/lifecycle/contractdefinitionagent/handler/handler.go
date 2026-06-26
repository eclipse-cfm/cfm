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

// Package handler implements the event processor for the contract definition lifecycle agent.
package handler

import (
	"context"

	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/system"
)

// ContractDefinitionEvent is the payload delivered on the "events.contract.definition.*" subjects.
type ContractDefinitionEvent struct {
	ID                 string `json:"id"`
	ParticipantContext string `json:"participantContextId"`
	AccessPolicyID     string `json:"accessPolicyId"`
	ContractPolicyID   string `json:"contractPolicyId"`
}

// Config holds the dependencies required by the Processor.
type Config struct {
	LogMonitor system.LogMonitor
}

// Processor reacts to contract definition lifecycle events.
type Processor struct {
	monitor system.LogMonitor
}

// NewProcessor constructs a contract definition event processor.
func NewProcessor(config *Config) *Processor {
	return &Processor{monitor: config.LogMonitor}
}

// Process handles a single contract definition event. Returning a recoverable error (see common/types) causes the
// message to be redelivered; any other error is treated as fatal and the message is dropped.
func (p *Processor) Process(_ context.Context, evt lifecycleagent.EventContext[ContractDefinitionEvent]) error {
	p.monitor.Infof("Received contract definition event on subject %s: id=%s participant=%s",
		evt.Subject, evt.Payload.ID, evt.Payload.ParticipantContext)

	// TODO: implement contract definition reaction logic here.
	return nil
}
