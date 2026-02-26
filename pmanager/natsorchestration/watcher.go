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

package natsorchestration

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
	"github.com/metaform/connector-fabric-manager/common/types"
	"github.com/metaform/connector-fabric-manager/pmanager/api"
	"github.com/nats-io/nats.go"
)

type MessageAck interface {
	Ack(opts ...nats.AckOpt) error
	Nak(opts ...nats.AckOpt) error
}

// OrchestrationIndexWatcher watches the underlying Jetsream KV subject for orchestration changes and updates the
// orchestration index. The Orchestration Index provides a query mechanism over orchestrations being processed as
// the Jetstream KV store is not optimized for queries. The Jetstream KV store is using an underlying stream and
// the watcher consumers update messages, recording relevant changes in the index.
type OrchestrationIndexWatcher struct {
	index            store.EntityStore[*api.OrchestrationEntry]
	trxContext       store.TransactionContext
	monitor          system.LogMonitor
	provisionManager api.ProvisionManager
}

func (w *OrchestrationIndexWatcher) onMessage(data []byte, msg MessageAck) {
	ctx := context.Background()

	var orchestration api.Orchestration
	err := json.Unmarshal(data, &orchestration)
	if err != nil {
		w.monitor.Infof("Failed to unmarshal orchestration entry: %v", err)
		_ = msg.Ack()
		return
	}

	_ = w.trxContext.Execute(ctx, func(ctx context.Context) error {
		currentEntry, err := w.index.FindByID(ctx, orchestration.ID)
		if err != nil && !errors.Is(err, types.ErrNotFound) {
			w.monitor.Infof("Failed to lookup orchestration entry: %v", err)
			_ = msg.Nak()
			return nil
		}

		entry := convertToEntry(orchestration)
		if currentEntry != nil { // Found
			// Only update if state and timestamp changed and not in a terminal state (messages may arrive out of order)
			if (currentEntry.State == orchestration.State && orchestration.StateTimestamp == currentEntry.StateTimestamp) ||
				currentEntry.State == api.OrchestrationStateCompleted ||
				currentEntry.State == api.OrchestrationStateErrored {
				return nil
			}
			entry.State = orchestration.State
			entry.StateTimestamp = orchestration.StateTimestamp
			if w.index.Update(ctx, entry) != nil {
				w.monitor.Infof("Failed to update orchestration entry: %v", err)
				_ = msg.Nak()
				return nil
			}

			// the orchestration failed, kick off the compensation
			if orchestration.State == api.OrchestrationStateErrored && orchestration.OrchestrationType != model.VPADisposeType {
				w.monitor.Infof("Orchestration [%s] is in state [%s] with error: [%s]. Starting auto-compensation", orchestration.ID, orchestration.State.String(), orchestration.ProcessingData["error"])
				correlationID := orchestration.CorrelationID //eg. participant profile id, etc.
				pData := orchestration.ProcessingData        // stuff for the VPAs
				manifest := model.OrchestrationManifest{
					ID:                uuid.NewString(),
					CorrelationID:     correlationID,
					OrchestrationType: model.VPADisposeType,
					Payload:           pData,
				}
				if w.provisionManager == nil {
					panic("Watcher: provisionManager is nil")
				}
				compensation, err := w.provisionManager.Start(ctx, &manifest)
				if err != nil {
					return err
				}
				w.monitor.Infof("Launching Orchestration [%s] as compensation for [%s]", compensation.ID, orchestration.ID)
			}

			// w.monitor.Debugf("Orchestration index entry %s updated to state %s", orchestration.ID, orchestration.State)
		} else {
			_, err := w.index.Create(ctx, entry)
			if err != nil {
				w.monitor.Infof("Failed to create orchestration entry: %v", err)
				_ = msg.Nak()
				return nil
			}
			// w.monitor.Debugf("Created orchestration index entry %s in state %s", orchestration.ID, orchestration.State)
		}
		if msg.Ack() != nil {
			w.monitor.Infof("Failed to acknowledge message for orchestration %s: %v", orchestration.ID, err)
		}
		return nil
	})
}

func convertToEntry(orchestration api.Orchestration) *api.OrchestrationEntry {
	entry := &api.OrchestrationEntry{
		ID:                orchestration.ID,
		OrchestrationType: orchestration.OrchestrationType,
		CorrelationID:     orchestration.CorrelationID,
		DefinitionID:      orchestration.DefinitionID,
		State:             orchestration.State,
		StateTimestamp:    orchestration.StateTimestamp,
		CreatedTimestamp:  orchestration.CreatedTimestamp,
	}
	return entry
}
