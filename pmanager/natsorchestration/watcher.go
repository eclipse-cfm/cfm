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
	"fmt"
	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// watcherRedeliveryDelay is how long a rejected orchestration update waits before it is redelivered. It keeps an update
// that keeps failing, for example while the database is unavailable, from being retried in a hot loop.
const watcherRedeliveryDelay = 5 * time.Second

// MessageAck acknowledges a JetStream message. It is implemented by jetstream.Msg.
type MessageAck interface {
	Ack() error
	NakWithDelay(delay time.Duration) error
}

// OrchestrationIndexWatcher watches the underlying Jetsream KV subject for orchestration changes and updates the
// orchestration index. The Orchestration Index provides a query mechanism over orchestrations being processed as
// the Jetstream KV store is not optimized for queries. The Jetstream KV store is using an underlying stream and
// the watcher consumers update messages, recording relevant changes in the index.
type OrchestrationIndexWatcher struct {
	index             store.EntityStore[*api.OrchestrationEntry]
	definitionManager api.DefinitionManager
	trxContext        store.TransactionContext
	monitor           system.LogMonitor
	provisionManager  api.ProvisionManager
}

// onMessage records an orchestration update in the index and starts a compensation when an orchestration has failed.
// The message is acknowledged only once the index transaction has committed. If the index cannot be read or written,
// or the compensation cannot be started, the transaction is rolled back and the message is rejected so that it is
// redelivered.
func (w *OrchestrationIndexWatcher) onMessage(data []byte, msg MessageAck) {
	ctx := context.Background()

	var orchestration api.Orchestration
	err := json.Unmarshal(data, &orchestration)
	if err != nil {
		w.monitor.Infof("Failed to unmarshal orchestration entry: %v", err)
		_ = msg.Ack()
		return
	}

	err = w.trxContext.Execute(ctx, func(ctx context.Context) error {
		currentEntry, err := w.index.FindByID(ctx, orchestration.ID)
		if err != nil && !errors.Is(err, types.ErrNotFound) {
			return fmt.Errorf("failed to look up orchestration entry: %w", err)
		}

		entry := convertToEntry(orchestration)
		if currentEntry == nil {
			if _, err := w.index.Create(ctx, entry); err != nil {
				return fmt.Errorf("failed to create orchestration entry: %w", err)
			}
			return nil
		}

		// Only update if state and timestamp changed and not in a terminal state (messages may arrive out of order)
		if (currentEntry.State == orchestration.State && orchestration.StateTimestamp == currentEntry.StateTimestamp) ||
			currentEntry.State == api.OrchestrationStateCompleted ||
			currentEntry.State == api.OrchestrationStateErrored {
			return nil
		}
		entry.State = orchestration.State
		entry.StateTimestamp = orchestration.StateTimestamp
		if err := w.index.Update(ctx, entry); err != nil {
			return fmt.Errorf("failed to update orchestration entry: %w", err)
		}

		// the orchestration failed, kick off the compensation
		if orchestration.State == api.OrchestrationStateErrored && orchestration.OrchestrationType != model.VPADisposeType {
			if err := w.startCompensationOrchestration(ctx, orchestration); err != nil {
				return fmt.Errorf("failed to start compensation: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		w.monitor.Warnf("Orchestration %s not indexed, it will be redelivered: %v", orchestration.ID, err)
		_ = msg.NakWithDelay(watcherRedeliveryDelay)
		return
	}
	if err := msg.Ack(); err != nil {
		w.monitor.Infof("Failed to acknowledge message for orchestration %s: %v", orchestration.ID, err)
	}
}

// CompensationID returns the ID of the compensation orchestration for the given orchestration. It is derived from the
// orchestration ID so that the compensation is started only once, even if the failure is handled more than once.
func CompensationID(orchestrationID string) string {
	return orchestrationID + "-compensation"
}

// startCompensationOrchestration initiates a compensation orchestration if a matching definition exists for the given orchestration.
// an error is returned if no compensation orchestration definition exists, if the orchestration could not be started, or if the exists-check failed
// de-duplication is handled by the ProvisionManager
func (w *OrchestrationIndexWatcher) startCompensationOrchestration(ctx context.Context, orchestration api.Orchestration) error {

	exists, err := w.existsCompensationOrchestration(ctx, model.VPADisposeType, orchestration.DefinitionID)
	if err != nil {
		return err
	}
	if !exists {
		w.monitor.Warnf("No compensation orchestration definition found for orchestration [%s]", orchestration.ID)
		return nil
	}

	// The KV watcher runs under a fresh context with no inbound message to extract trace context from, so
	// without this the compensation would start a brand-new, disconnected trace. Link it back to the
	// originating trace of the failed orchestration. A link (rather than a parent) is used because
	// compensation is a reactive saga step, not a continuation of the original request.
	var links []trace.Link
	if len(orchestration.OriginTraceContext) > 0 {
		originCtx := otel.GetTextMapPropagator().Extract(context.Background(), propagation.MapCarrier(orchestration.OriginTraceContext))
		links = append(links, trace.LinkFromContext(originCtx))
	}
	ctx, span := otel.GetTracerProvider().Tracer("cfm.pmanager.orchestrator").Start(ctx, "nats.auto_compensation", trace.WithLinks(links...))
	defer span.End()

	w.monitor.Infof("Orchestration [%s] is in state [%s] with error: [%s]. Starting auto-compensation", orchestration.ID, orchestration.State.String(), orchestration.EngineState.Error)

	correlationID := orchestration.CorrelationID //eg. participant profile id, etc.
	pData := orchestration.ProcessingData        // stuff for the VPAs
	manifest := model.OrchestrationManifest{
		ID:                    CompensationID(orchestration.ID),
		CorrelationID:         correlationID,
		OrchestrationType:     model.VPADisposeType,
		Payload:               pData,
		CompensatedManifestID: orchestration.ID,
	}
	compensation, err := w.provisionManager.Start(ctx, &manifest)
	if err != nil {
		return err
	}
	w.monitor.Infof("Launching Orchestration [%s] as compensation for [%s]", compensation.ID, orchestration.ID)
	return nil
}

// existsCompensationOrchestration verifies that another orchestration definition exists that was generated based on the same template
// and that has the desired orchestration type
func (w *OrchestrationIndexWatcher) existsCompensationOrchestration(ctx context.Context, orchestrationType model.OrchestrationType, orchestrationDefinitionID string) (bool, error) {
	// get orchestration definition by ID
	idPredicate := query.Eq("type", orchestrationDefinitionID)
	definitions, err := w.definitionManager.QueryOrchestrationDefinitions(ctx, idPredicate)
	if err != nil {
		return false, err
	}
	if len(definitions) != 1 {
		return false, types.NewFatalError("expected exactly one orchestration definition with ID %s, found %d", orchestrationDefinitionID, len(definitions))
	}
	templateRef := definitions[0].TemplateRef
	templateRefPredicate := query.And(query.Eq("templateRef", templateRef), query.Eq("type", orchestrationType.String()))
	compensationDefinitions, err := w.definitionManager.QueryOrchestrationDefinitions(ctx, templateRefPredicate)

	return len(compensationDefinitions) == 1, err
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
