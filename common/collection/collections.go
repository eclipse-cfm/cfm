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

package collection

import (
	"iter"
	"slices"
)

// CollectAll collects all sequence elements into a slice.
func CollectAll[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var result []T
	for item, err := range seq {
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if result == nil {
		return []T{}, nil
	}
	return result, nil
}

func CollectAllDeref[T any](seq iter.Seq2[*T, error]) ([]T, error) {
	var result []T
	for item, err := range seq {
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	if result == nil {
		return []T{}, nil
	}
	return result, nil
}

// DerefSlice dereferences slice entry points.
func DerefSlice[T any](ptrs []*T) []T {
	values := make([]T, 0, len(ptrs))
	for _, ptr := range ptrs {
		if ptr != nil {
			values = append(values, *ptr)
		}
	}
	return values
}

// From convert a slice of objects to an iter.Seq2
func From[T any](slice []T) iter.Seq[T] {
	return slices.Values(slice)
}

// Map apply a function to each element of the sequence and return a new sequence with the results
func Map[T any, R any](seq iter.Seq[T], fn func(T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for v := range seq {
			if !yield(fn(v)) {
				return
			}
		}
	}
}

// Distinct returns a new sequence with only unique elements
func Distinct[T any](seq iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		seen := make(map[any]struct{})
		for v := range seq {
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			if !yield(v) {
				return
			}
		}
	}
}

// Collect returns a slice with all elements of the sequence
func Collect[T any](seq iter.Seq[T]) []T {
	var out []T
	for v := range seq {
		out = append(out, v)
	}
	return out
}
