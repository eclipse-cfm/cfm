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

package natsclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

const CFMSubjectPrefix = "event"
const CFMOrchestration = "cfm-orchestration"
const CFMOrchestrationSubject = CFMSubjectPrefix + "." + CFMOrchestration
const CFMOrchestrationResponse = "cfm-orchestration-response"
const CFMOrchestrationResponseSubject = CFMSubjectPrefix + "." + CFMOrchestrationResponse

// SetupStream configures a JetStream stream used for component messaging. If the stream does not exist, it is created.
func SetupStream(ctx context.Context, client *NatsClient, streamName string) (jetstream.Stream, error) {
	stream, err := client.JetStream.Stream(ctx, streamName)
	if err == nil {
		return stream, nil
	}

	// If stream doesn't exist, create it
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		cfg := jetstream.StreamConfig{
			Name:      streamName,
			Retention: jetstream.WorkQueuePolicy,
			Subjects:  []string{CFMSubjectPrefix + ".*"},
		}
		return client.JetStream.CreateOrUpdateStream(ctx, cfg)
	}

	return nil, fmt.Errorf("unable to access NATS stream: %w", err)
}

// SetupConsumer creates or updates a NATS JetStream consumer for an activity processor.
func SetupConsumer(ctx context.Context, stream jetstream.Stream, subject string) (jetstream.Consumer, error) {
	sanitizedSubject := strings.ReplaceAll(subject, ".", "-") // convert to `-` because NATs uses dot-notation to denote subject hierarchies
	return stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       sanitizedSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: CFMSubjectPrefix + "." + sanitizedSubject,
	})
}

// SetupStreamWithSubjects creates or updates a JetStream stream that carries arbitrary, configurable subjects (for
// example "events.contract.definition.created", including "*" and ">" wildcards). Subjects are used verbatim, with no
// prefixing or sanitization.
//
// Unlike SetupStream, it uses jetstream.InterestPolicy rather than WorkQueuePolicy. A WorkQueue stream deletes a message
// once any single consumer acknowledges it and binds each subject to a single consumer, so only one agent could ever
// react to a given event. InterestPolicy instead retains a message until all interested consumers have acknowledged it,
// allowing multiple lifecycle agents to fan out from the same event. At least one subject must be provided.
//
// Note: InterestPolicy retains a message only while a consumer has interest in it, so events published before any
// matching durable consumer exists are not retained.
func SetupStreamWithSubjects(ctx context.Context, client *NatsClient, streamName string, subjects []string) (jetstream.Stream, error) {
	if len(subjects) == 0 {
		return nil, fmt.Errorf("cannot set up stream %q: at least one subject must be provided", streamName)
	}

	_, err := client.JetStream.Stream(ctx, streamName)
	if err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
		return nil, fmt.Errorf("unable to access NATS stream: %w", err)
	}

	return client.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      streamName,
		Retention: jetstream.InterestPolicy,
		Storage:   jetstream.MemoryStorage,
		Subjects:  subjects,
	})
}

// SetupMultiSubjectConsumer creates or updates a single durable consumer bound to all the given filter subjects. The
// subjects are used verbatim (no prefixing or sanitization), and the durable name is derived from the agent rather than
// the subject so that one consumer can span multiple subjects.
func SetupMultiSubjectConsumer(ctx context.Context, stream jetstream.Stream, durable string, subjects []string) (jetstream.Consumer, error) {
	return stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:        durable,
		AckPolicy:      jetstream.AckExplicitPolicy,
		FilterSubjects: subjects,
	})
}
