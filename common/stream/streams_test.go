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
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFrom(t *testing.T) {
	t.Run("should create stream from slice", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		stream := From(slice)

		result := stream.Collect()

		assert.Equal(t, slice, result)
	})

	t.Run("should create stream from empty slice", func(t *testing.T) {
		slice := []int{}
		stream := From(slice)

		result := stream.Collect()

		assert.Empty(t, result)
	})

	t.Run("should work with different types", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		stream := From(slice)

		result := stream.Collect()

		assert.Equal(t, slice, result)
	})
}

func TestStream_Seq(t *testing.T) {
	t.Run("should return iterable sequence", func(t *testing.T) {
		slice := []int{1, 2, 3}
		stream := From(slice)

		var result []int
		for v := range stream.Seq() {
			result = append(result, v)
		}

		assert.Equal(t, slice, result)
	})
}

func TestMap(t *testing.T) {
	t.Run("should map int to string", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		stream := From(slice)
		mapped := Map(stream, func(i int) string {
			return strconv.Itoa(i)
		})
		result := mapped.Collect()
		assert.Equal(t, []string{"1", "2", "3", "4", "5"}, result)
	})

	t.Run("should map int to int with transformation", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		stream := From(slice)
		mapped := Map(stream, func(i int) int {
			return i * 2
		})
		result := mapped.Collect()

		assert.Equal(t, []int{2, 4, 6, 8, 10}, result)
	})

	t.Run("should work with empty stream", func(t *testing.T) {
		slice := []int{}
		stream := From(slice)
		mapped := Map(stream, func(i int) string {
			return strconv.Itoa(i)
		})
		result := mapped.Collect()

		assert.Empty(t, result)
	})

	t.Run("should map string to struct", func(t *testing.T) {
		type Person struct {
			Name string
		}

		slice := []string{"Alice", "Bob", "Charlie"}
		stream := From(slice)

		mapped := Map(stream, func(name string) Person {
			return Person{Name: name}
		})
		result := mapped.Collect()

		expected := []Person{
			{Name: "Alice"},
			{Name: "Bob"},
			{Name: "Charlie"},
		}
		assert.Equal(t, expected, result)
	})

	t.Run("should handle early termination", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		stream := From(slice)
		mapped := Map(stream, func(i int) int {
			return i * 2
		})

		var result []int
		for v := range mapped.Seq() {
			result = append(result, v)
			if v == 4 {
				break
			}
		}

		assert.Equal(t, []int{2, 4}, result)
	})
}

func TestStream_Distinct(t *testing.T) {
	t.Run("should remove duplicate integers", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 3, 4, 5, 5}
		stream := From(slice)

		distinct := stream.Distinct()
		result := distinct.Collect()

		assert.ElementsMatch(t, []int{1, 2, 3, 4, 5}, result)
	})

	t.Run("should remove duplicate strings", func(t *testing.T) {
		slice := []string{"a", "b", "a", "c", "b"}
		stream := From(slice)

		distinct := stream.Distinct()
		result := distinct.Collect()

		assert.ElementsMatch(t, []string{"a", "b", "c"}, result)
	})

	t.Run("should return same elements when all are unique", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		stream := From(slice)

		distinct := stream.Distinct()
		result := distinct.Collect()

		assert.Equal(t, slice, result)
	})

	t.Run("should work with empty stream", func(t *testing.T) {
		slice := []int{}
		stream := From(slice)

		distinct := stream.Distinct()
		result := distinct.Collect()

		assert.Empty(t, result)
	})

	t.Run("should handle early termination", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 4, 4, 5}
		stream := From(slice)

		distinct := stream.Distinct()

		var result []int
		for v := range distinct.Seq() {
			result = append(result, v)
			if v == 3 {
				break
			}
		}

		assert.Equal(t, []int{1, 2, 3}, result)
	})
}

func TestStream_Collect(t *testing.T) {
	t.Run("should collect all elements", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		stream := From(slice)

		result := stream.Collect()

		assert.Equal(t, slice, result)
	})

	t.Run("should collect empty stream", func(t *testing.T) {
		slice := []int{}
		stream := From(slice)

		result := stream.Collect()

		assert.Empty(t, result)
	})
}

func TestStream_ChainedOperations(t *testing.T) {
	t.Run("should chain Map and Distinct", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 4}
		stream := From(slice)

		result := Map(stream, func(i int) int {
			return i * 2
		}).Distinct().Collect()

		assert.ElementsMatch(t, []int{2, 4, 6, 8}, result)
	})

	t.Run("should chain Distinct and Map", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 4}
		stream := From(slice)

		distinct := stream.Distinct()
		result := Map(distinct, func(i int) string {
			return strconv.Itoa(i * 10)
		}).Collect()

		assert.ElementsMatch(t, []string{"10", "20", "30", "40"}, result)
	})

	t.Run("should chain multiple operations", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 4, 5, 5}
		stream := From(slice)

		result := Map(
			stream.Distinct(),
			func(i int) int { return i * 2 },
		).Distinct().Collect()

		assert.ElementsMatch(t, []int{2, 4, 6, 8, 10}, result)
	})
}
