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
	"strings"

	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
)

// EnqueueActivityMessages enqueues the given activities for processing.
//
// Messages are sent to a named durable queue corresponding to the activity type. For example, messages for the
// 'test-activity' type will be routed to the 'event.test-activity' queue.
//
// Each message carries a message ID derived from the orchestration and activity IDs, so JetStream drops a message that
// is enqueued again within the stream's duplicate window, for example when an enqueue is retried after a failure.
func EnqueueActivityMessages(ctx context.Context, orchestrationID string, activities []api.Activity, client natsclient.MsgClient) error {
	for _, activity := range activities {
		// route to queue
		payload, err := json.Marshal(api.ActivityMessage{
			OrchestrationID: orchestrationID,
			Activity:        activity,
		})
		if err != nil {
			return fmt.Errorf("error marshalling activity payload: %w", err)
		}

		// Strip out periods since they denote a subject hierarchy for NATS
		subject := natsclient.CFMSubjectPrefix + "." + strings.ReplaceAll(activity.Type.String(), ".", "-")
		headers := nats.Header{}
		headers.Set(jetstream.MsgIDHeader, ActivityMessageID(orchestrationID, activity.ID))
		otel.GetTextMapPropagator().Inject(ctx, &natsHeaderCarrier{headers: headers})

		msg := &nats.Msg{
			Subject: subject,
			Data:    payload,
			Header:  headers,
		}
		_, err = client.PublishMsg(ctx, msg)
		if err != nil {
			return fmt.Errorf("error publishing to stream: %w", err)
		}
	}
	return nil
}

// ActivityMessageID returns the JetStream message ID of the message for an activity of an orchestration.
func ActivityMessageID(orchestrationID string, activityID string) string {
	return orchestrationID + "." + activityID
}

// ReadOrchestration reads the orchestration state from the KV store.
func ReadOrchestration(ctx context.Context, orchestrationID string, client natsclient.MsgClient) (api.Orchestration, uint64, error) {
	oEntry, err := client.Get(ctx, orchestrationID)
	if err != nil {
		return api.Orchestration{}, 0, fmt.Errorf("failed to get orchestration state %s: %w", orchestrationID, err)
	}

	var orchestration api.Orchestration
	if err = json.Unmarshal(oEntry.Value(), &orchestration); err != nil {
		return api.Orchestration{}, 0, fmt.Errorf("failed to unmarshal orchestration %s: %w", orchestrationID, err)
	}

	return orchestration, oEntry.Revision(), nil
}

// maxUpdateConflicts is how often UpdateOrchestration re-reads and retries an update that lost a race against a
// concurrent update, before giving up.
const maxUpdateConflicts = 50

// UpdateOrchestration updates the orchestration state in the KV store using optimistic concurrency by comparing the
// last known revision. If another update got there first, the orchestration is re-read and updateFn is applied again,
// up to maxUpdateConflicts times. Any other error is returned straight away, so that callers can retry later instead of
// spinning on a KV store that cannot take the write.
func UpdateOrchestration(
	ctx context.Context,
	orchestration api.Orchestration,
	revision uint64,
	client natsclient.MsgClient,
	updateFn func(*api.Orchestration)) (api.Orchestration, uint64, error) {
	for conflicts := 0; ; conflicts++ {
		updateFn(&orchestration)
		serialized, err := json.Marshal(orchestration)
		if err != nil {
			return api.Orchestration{}, 0, fmt.Errorf("failed to marshal orchestration %s: %w", orchestration.ID, err)
		}
		newRevision, err := client.Update(ctx, orchestration.ID, serialized, revision)
		if err == nil {
			return orchestration, newRevision, nil
		}
		if !isRevisionConflict(err) {
			return api.Orchestration{}, 0, fmt.Errorf("failed to update orchestration %s: %w", orchestration.ID, err)
		}
		if conflicts+1 >= maxUpdateConflicts {
			return api.Orchestration{}, 0, fmt.Errorf("failed to update orchestration %s after %d conflicting updates: %w",
				orchestration.ID, maxUpdateConflicts, err)
		}
		orchestration, revision, err = ReadOrchestration(ctx, orchestration.ID, client)
		if err != nil {
			return api.Orchestration{}, 0, fmt.Errorf("failed to read orchestration data for update: %w", err)
		}
	}
}

// isRevisionConflict reports whether a KV update failed because the entry was changed since the given revision was read
// (or, for revision 0, because the entry already exists).
func isRevisionConflict(err error) bool {
	var jsErr *jetstream.APIError
	return errors.As(err, &jsErr) && jsErr.ErrorCode == jetstream.JSErrCodeStreamWrongLastSequence
}
