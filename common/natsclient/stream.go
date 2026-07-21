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

// GetStream looks up an existing JetStream stream by name. If the stream does not exist, it creates it with default
// configuration. Adding the agent's own subjects to an existing stream is done separately via EnsureStreamSubjects.
func GetStream(ctx context.Context, client *NatsClient, streamName string, subjects []string) (jetstream.Stream, error) {
	stream, err := client.JetStream.Stream(ctx, streamName)

	if err == nil {
		// make sure the desired subjects are added to the stream
		return EnsureStreamSubjects(ctx, client, stream, subjects)
	}
	// If stream doesn't exist, create it with the desired subjects
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		cfg := jetstream.StreamConfig{
			Name:      streamName,
			Retention: jetstream.InterestPolicy,
			Storage:   jetstream.MemoryStorage,
			Subjects:  subjects,
		}
		// no need to call EnsureSteamSubjects, because we are creating the stream with the desired subjects
		return client.JetStream.CreateStream(ctx, cfg)
	}
	return nil, fmt.Errorf("unable to access NATS stream %q: %w", streamName, err)
}

// GetExistingStream looks up an existing JetStream stream by name without creating it. It returns an
// error if the stream does not exist, so callers that rely on an externally-managed stream fail fast
// until it is present. Unlike GetStream it does not create the stream or reconcile its subjects.
func GetExistingStream(ctx context.Context, client *NatsClient, streamName string) (jetstream.Stream, error) {
	stream, err := client.JetStream.Stream(ctx, streamName)
	if err != nil {
		return nil, fmt.Errorf("unable to access NATS stream %q: %w", streamName, err)
	}
	return stream, nil
}

// EnsureStreamSubjects adds the given subjects to an existing stream so the stream carries the events the agent consumes.
//
// NATS rejects a stream whose subjects overlap, so the subjects cannot simply be unioned: registering
// "events.something" against a stream that already has "events.>" would create an overlap. Subjects are therefore merged
// by coverage (see mergeStreamSubjects): a subject already covered by an existing one is dropped, a subject that is
// broader than existing ones replaces them, and genuinely disjoint subjects are appended. If the merge leaves the stream
// subjects unchanged, the stream is returned as-is; otherwise the stream is updated in place.
func EnsureStreamSubjects(ctx context.Context, client *NatsClient, stream jetstream.Stream, subjects []string) (jetstream.Stream, error) {
	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to read configuration of stream %q: %w", stream.CachedInfo().Config.Name, err)
	}

	merged, changed, err := mergeStreamSubjects(info.Config.Subjects, subjects)
	if err != nil {
		return nil, fmt.Errorf("cannot add subjects to stream %q: %w", info.Config.Name, err)
	}
	if !changed {
		return stream, nil
	}

	cfg := info.Config
	cfg.Subjects = merged
	updated, err := client.JetStream.UpdateStream(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to update subjects of stream %q: %w", info.Config.Name, err)
	}
	return updated, nil
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

// mergeStreamSubjects merges additional subjects into the existing stream subjects without creating the overlaps NATS
// prohibits. It returns the merged set and whether it differs from existing. For each additional subject:
//   - if an existing subject already covers it, it is redundant and skipped;
//   - if it covers existing subjects, those narrower subjects are removed and it is added in their place;
//   - if it is disjoint from all existing subjects, it is appended;
//   - if it partially overlaps an existing subject without either covering the other (for example "a.*" and "*.b"), the
//     subjects cannot coexist in one stream and an error is returned.
func mergeStreamSubjects(existing, additional []string) ([]string, bool, error) {
	result := append([]string(nil), existing...)
	changed := false

	for _, a := range additional {
		if anySubjectCovers(result, a) {
			continue // already carried by an existing, broader-or-equal subject
		}

		kept := make([]string, 0, len(result))
		for _, e := range result {
			if subjectCovers(a, e) {
				changed = true // a is broader and supersedes e
				continue
			}
			if subjectsCollide(a, e) {
				return nil, false, fmt.Errorf("subject %q partially overlaps existing stream subject %q and cannot be merged", a, e)
			}
			kept = append(kept, e)
		}
		result = append(kept, a)
		changed = true
	}

	return result, changed, nil
}

func anySubjectCovers(subjects []string, subject string) bool {
	for _, s := range subjects {
		if subjectCovers(s, subject) {
			return true
		}
	}
	return false
}

// subjectCovers reports whether every concrete subject matching sub also matches super, i.e. super is at least as broad
// as sub. Tokens follow NATS semantics: "*" matches exactly one token and ">" matches one or more trailing tokens.
func subjectCovers(super, sub string) bool {
	st := strings.Split(super, ".")
	ft := strings.Split(sub, ".")
	si, fi := 0, 0
	for {
		if si == len(st) {
			return fi == len(ft) // covered only if both are fully consumed
		}
		if st[si] == ">" {
			return fi < len(ft) // ">" matches one or more remaining tokens
		}
		if fi == len(ft) {
			return false // super requires more tokens than sub produces
		}
		if st[si] == "*" {
			if ft[fi] == ">" {
				return false // "*" matches exactly one token; ">" may expand to several
			}
		} else if st[si] != ft[fi] {
			return false // literal mismatch, or sub token is a wildcard broader than the literal
		}
		si++
		fi++
	}
}

// subjectsCollide reports whether some concrete subject matches both a and b. It is used to detect a partial overlap
// when neither subject covers the other.
func subjectsCollide(a, b string) bool {
	at := strings.Split(a, ".")
	bt := strings.Split(b, ".")
	for i := 0; ; i++ {
		aDone, bDone := i == len(at), i == len(bt)
		if aDone && bDone {
			return true
		}
		if aDone || bDone {
			return false // one ran out of tokens with no ">" to absorb the rest
		}
		if at[i] == ">" || bt[i] == ">" {
			return true // ">" absorbs the remainder of the other subject
		}
		if at[i] != "*" && bt[i] != "*" && at[i] != bt[i] {
			return false // two literals that differ cannot both match
		}
	}
}
