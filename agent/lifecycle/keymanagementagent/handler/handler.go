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
	"errors"
	"net/http"

	"github.com/eclipse-cfm/cfm/agent/common/controlplane"
	"github.com/eclipse-cfm/cfm/agent/common/siglet"
	"github.com/eclipse-cfm/cfm/common/httpx"
	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
)

// KeyManagementEvent is the CloudEvent delivered on the "events.key.management.*" subjects.
type KeyManagementEvent = lifecycleagent.CloudEvent[KeyPairEventData]

// EDC control plane STS signature config keys set on key activation.
const (
	stsTypeKey       = "edc.iam.sts.type"
	stsTypeSignature = "signature"
	stsKeyNameKey    = "edc.iam.sts.signature.keyname"
	stsKidKey        = "edc.iam.sts.signature.kid"
)

// Config holds the dependencies required by the Processor.
type Config struct {
	LogMonitor         system.LogMonitor
	SigletClient       siglet.ManagementAPIClient
	ControlPlaneClient controlplane.ManagementAPIClient
}

// Processor reacts to key management lifecycle events.
type Processor struct {
	monitor            system.LogMonitor
	sigletClient       siglet.ManagementAPIClient
	controlPlaneClient controlplane.ManagementAPIClient
}

// NewProcessor constructs a key management event processor.
func NewProcessor(config *Config) *Processor {
	return &Processor{
		monitor:            config.LogMonitor,
		sigletClient:       config.SigletClient,
		controlPlaneClient: config.ControlPlaneClient,
	}
}

// Process handles a single key management event. Returning a client or fatal error (see common/types) drops the
// message; any other error causes it to be redelivered, up to the consumer's delivery limit. The Siglet and control
// plane clients classify their errors, so an unreachable or failing downstream is retried, while a request that was
// rejected for good is not. Dropping a revocation would leave the revoked key in use, so when in doubt, retry.
func (p *Processor) Process(ctx context.Context, evt lifecycleagent.EventContext[KeyManagementEvent]) error {

	keyEventData := evt.Payload.Data
	p.monitor.Infof("Received key management event %s", evt.Subject)

	switch evt.Subject {
	case "events.keypair.rotated":
	case "events.keypair.revoked":
		if err := validateParticipantContextID(evt.Subject, keyEventData); err != nil {
			return err
		}
		if err := p.handleKeyDecommissioned(ctx, keyEventData); err != nil {
			return err
		}
	case "events.keypair.activated":
		if err := validateParticipantContextID(evt.Subject, keyEventData); err != nil {
			return err
		}
		if err := p.handleKeyActivated(ctx, keyEventData); err != nil {
			return err
		}
		p.monitor.Debugf("Key '%s' activated for participant '%s': %s", keyEventData.KeyID, keyEventData.ParticipantContextID, keyEventData.PublicKeySerialized)
	default:
		return nil
	}

	return nil
}

// validateParticipantContextID rejects events whose participant context ID is not a plain identifier, since the ID ends
// up in admin API paths. The returned error is a client error, so the message is dropped instead of being redelivered.
func validateParticipantContextID(subject string, data KeyPairEventData) error {
	if err := model.ValidateParticipantContextID(data.ParticipantContextID); err != nil {
		return types.NewClientWrappedError(err, "rejecting key management event %s", subject)
	}
	return nil
}

// handleKeyActivated processes key activation events by associating the key with the participant on the
// EDC control plane and creating or updating key mappings in Siglet.
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

	// associate the activated key with the participant config on the control plane before touching Siglet
	if err := p.updateControlPlaneKeyAssociation(ctx, keyEventData); err != nil {
		return err
	}

	return nil
}

// updateControlPlaneKeyAssociation fetches the participant config from the control plane and, when present,
// patches it with the STS signature key metadata for the activated key. The config may not yet be
// provisioned when the activation event arrives (a race with the edcv agent), so a missing config is
// treated as a recoverable error to have the message redelivered and retried.
func (p *Processor) updateControlPlaneKeyAssociation(ctx context.Context, keyEventData KeyPairEventData) error {
	pid := keyEventData.ParticipantContextID

	patch := controlplane.ParticipantContextConfig{
		ParticipantContextID: pid,
		Entries: map[string]string{
			stsTypeKey:    stsTypeSignature,
			stsKeyNameKey: keyEventData.KeyPairResource.PrivateKeyAlias,
			stsKidKey:     keyEventData.KeyID,
		},
		SecretEntries: map[string]string{},
	}
	if err := p.controlPlaneClient.PatchConfig(ctx, pid, patch); err != nil {
		p.monitor.Warnf("Error patching participant config in control plane: %s", err)
		var statusErr *httpx.StatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
			return types.NewRecoverableWrappedError(err, "participant config %s not provisioned yet", pid)
		}
		return err
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
