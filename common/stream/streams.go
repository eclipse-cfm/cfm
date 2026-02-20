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

package stream

import (
	"iter"
	"slices"
)

// Stream simple streaming interface to execute sequential operations on a series (slice) of elements
type Stream[T any] struct {
	seq iter.Seq[T]
}

// From convert a slice of objects to a stream
func From[T any](slice []T) Stream[T] {
	return Stream[T]{
		seq: slices.Values(slice),
	}
}

// Seq return the stream as iter.Seq, e.g., for range operations
func (s Stream[T]) Seq() iter.Seq[T] {
	return s.seq
}

// Map apply a function to each element of the stream and return a new stream with the results. Note that due to a
// Go language limitation (methods can't have type parameters), this is a function
func Map[T any, R any](s Stream[T], fn func(T) R) Stream[R] {
	return Stream[R]{
		seq: func(yield func(R) bool) {

			for v := range s.seq {
				if !yield(fn(v)) {
					return
				}
			}
		},
	}
}

// Distinct returns a new stream with only unique elements
func (s Stream[T]) Distinct() Stream[T] {

	return Stream[T]{
		seq: func(yield func(T) bool) {
			seen := make(map[any]struct{})
			for v := range s.seq {

				if _, ok := seen[v]; ok {
					continue
				}
				seen[v] = struct{}{}
				if !yield(v) {
					return
				}
			}
		},
	}
}

// Collect returns a slice with all elements of the stream
func (s Stream[T]) Collect() []T {
	var out []T
	for v := range s.seq {
		out = append(out, v)
	}
	return out
}
