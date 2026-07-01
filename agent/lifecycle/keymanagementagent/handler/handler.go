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

// Package handler implements the event processor for the key management lifecycle agent.
package handler

import (
	"context"

	"github.com/eclipse-cfm/cfm/agent/lifecycle/keymanagementagent/siglet"
	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/system"
)

// KeyManagementEvent is the CloudEvent delivered on the "events.key.management.*" subjects.
type KeyManagementEvent = lifecycleagent.CloudEvent[KeyPairEventData]

// Config holds the dependencies required by the Processor.
type Config struct {
	LogMonitor   system.LogMonitor
	SigletClient siglet.ManagementAPIClient
}

// Processor reacts to key management lifecycle events.
type Processor struct {
	monitor      system.LogMonitor
	sigletClient siglet.ManagementAPIClient
}

// NewProcessor constructs a key management event processor.
func NewProcessor(config *Config) *Processor {
	return &Processor{monitor: config.LogMonitor, sigletClient: config.SigletClient}
}

// Process handles a single key management event. Returning a recoverable error (see common/types) causes the
// message to be redelivered; any other error is treated as fatal and the message is dropped.
func (p *Processor) Process(ctx context.Context, evt lifecycleagent.EventContext[KeyManagementEvent]) error {

	keyEventData := evt.Payload.Data
	p.monitor.Infof("Received key management event %s", evt.Subject)

	switch evt.Subject {
	case "events.keypair.rotated":
	case "events.keypair.revoked":
		if err := p.handleKeyDecommissioned(ctx, keyEventData); err != nil {
			return err
		}
	case "events.keypair.activated":
		if err := p.handleKeyActivated(ctx, keyEventData); err != nil {
			return err
		}
		p.monitor.Debugf("Key '%s' activated for participant '%s': %s", keyEventData.KeyID, keyEventData.ParticipantContextID, keyEventData.PublicKeySerialized)
	default:
		return nil
	}

	return nil
}

// handleKeyActivated processes key activation events by creating or updating key mappings in Siglet
func (p *Processor) handleKeyActivated(ctx context.Context, keyEventData KeyPairEventData) error {
	mapping := siglet.KeyMapping{
		ParticipantContextID: keyEventData.ParticipantContextID,
		KeyName:              keyEventData.KeyPairResource.PrivateKeyAlias,
		KeyID:                keyEventData.KeyID,
	}

	// get key mapping, see if already exists
	km, err := p.sigletClient.GetKeyMapping(ctx, mapping.ParticipantContextID)
	if err != nil {
		p.monitor.Warnf("Error getting key mapping from siglet: %s", err)
		return err
	}

	if km != nil {
		p.monitor.Debugf("Key mapping already exists in siglet, updating: %s", km)
		if err := p.sigletClient.UpdateKeyMapping(ctx, mapping); err != nil {
			p.monitor.Warnf("Error updating key mapping in siglet: %s", err)
			return err
		}
	} else {
		if err := p.sigletClient.CreateKeyMapping(ctx, mapping); err != nil {
			p.monitor.Warnf("Error creating key mapping in siglet: %s", err)
			return err
		}
	}
	return nil
}

// handleKeyDecommissioned used when keys are "decommissioned", i.e., revoked or rotated. The old key is deleted from Siglet
// notably, no new mapping is created, because for the new key, another "activated" event will be sent.
func (p *Processor) handleKeyDecommissioned(ctx context.Context, data KeyPairEventData) error {
	participantContextID := data.ParticipantContextID

	if err := p.sigletClient.DeleteKeyMapping(ctx, participantContextID); err != nil {
		p.monitor.Warnf("Error deleting key mapping from siglet: %s", err)
		return err
	}
	return nil
}
