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
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectAll(t *testing.T) {
	t.Run("empty sequence", func(t *testing.T) {
		seq := func(yield func(int, error) bool) {
			// Empty sequence - don't yield anything
		}

		result, err := CollectAll(seq)

		require.NoError(t, err)
		assert.Empty(t, result)
		assert.NotNil(t, result) // Should be empty slice, not nil
	})

	t.Run("multiple items sequence", func(t *testing.T) {
		items := []int{1, 2, 3, 4, 5}
		seq := func(yield func(int, error) bool) {
			for _, item := range items {
				if !yield(item, nil) {
					break
				}
			}
		}

		result, err := CollectAll(seq)

		require.NoError(t, err)
		require.Len(t, result, 5)
		assert.Equal(t, items, result)
	})

	t.Run("sequence with error at beginning", func(t *testing.T) {
		expectedErr := errors.New("test error")
		seq := func(yield func(string, error) bool) {
			yield("", expectedErr)
		}

		result, err := CollectAll(seq)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("sequence with error in middle", func(t *testing.T) {
		expectedErr := errors.New("middle error")
		seq := func(yield func(string, error) bool) {
			if !yield("item1", nil) {
				return
			}
			if !yield("item2", nil) {
				return
			}
			if !yield("", expectedErr) {
				return
			} // Error in the middle
			if !yield("item4", nil) {
				return
			} // This should not be reached
		}

		result, err := CollectAll(seq)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result) // Should return nil on error
	})

	t.Run("sequence with error at end", func(t *testing.T) {
		expectedErr := errors.New("end error")
		seq := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
			yield(3, nil)
			yield(0, expectedErr) // Error at end
		}

		result, err := CollectAll(seq)

		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("sequence with nil pointer values", func(t *testing.T) {
		type TestEntity struct {
			Value string
		}

		seq := func(yield func(*TestEntity, error) bool) {
			yield(nil, nil)                        // Nil pointer
			yield(&TestEntity{Value: "test"}, nil) // Valid pointer
			yield(nil, nil)                        // Nil pointer again
		}

		result, err := CollectAll(seq)

		require.NoError(t, err)
		require.Len(t, result, 3)
		assert.Nil(t, result[0])
		assert.NotNil(t, result[1])
		assert.Equal(t, "test", result[1].Value)
		assert.Nil(t, result[2])
	})
}

func TestFrom(t *testing.T) {
	t.Run("should create sequence from slice", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		seq := From(slice)

		result := Collect(seq)

		assert.Equal(t, slice, result)
	})

	t.Run("should create sequence from empty slice", func(t *testing.T) {
		slice := []int{}
		seq := From(slice)

		result := Collect(seq)

		assert.Empty(t, result)
	})

	t.Run("should work with different types", func(t *testing.T) {
		slice := []string{"a", "b", "c"}
		seq := From(slice)

		result := Collect(seq)

		assert.Equal(t, slice, result)
	})
}

func TestSeq_Iteration(t *testing.T) {
	t.Run("should iterate over sequence", func(t *testing.T) {
		slice := []int{1, 2, 3}
		seq := From(slice)

		var result []int
		for v := range seq {
			result = append(result, v)
		}

		assert.Equal(t, slice, result)
	})
}

func TestMap(t *testing.T) {
	t.Run("should map int to string", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		seq := From(slice)
		mapped := Map(seq, func(i int) string {
			return strconv.Itoa(i)
		})
		result := Collect(mapped)
		assert.Equal(t, []string{"1", "2", "3", "4", "5"}, result)
	})

	t.Run("should map int to int with transformation", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		seq := From(slice)
		mapped := Map(seq, func(i int) int {
			return i * 2
		})
		result := Collect(mapped)

		assert.Equal(t, []int{2, 4, 6, 8, 10}, result)
	})

	t.Run("should work with empty sequence", func(t *testing.T) {
		slice := []int{}
		seq := From(slice)
		mapped := Map(seq, func(i int) string {
			return strconv.Itoa(i)
		})
		result := Collect(mapped)

		assert.Empty(t, result)
	})

	t.Run("should map string to struct", func(t *testing.T) {
		type Person struct {
			Name string
		}

		slice := []string{"Alice", "Bob", "Charlie"}
		seq := From(slice)

		mapped := Map(seq, func(name string) Person {
			return Person{Name: name}
		})
		result := Collect(mapped)

		expected := []Person{
			{Name: "Alice"},
			{Name: "Bob"},
			{Name: "Charlie"},
		}
		assert.Equal(t, expected, result)
	})

	t.Run("should handle early termination", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		seq := From(slice)
		mapped := Map(seq, func(i int) int {
			return i * 2
		})

		var result []int
		for v := range mapped {
			result = append(result, v)
			if v == 4 {
				break
			}
		}

		assert.Equal(t, []int{2, 4}, result)
	})
}

func TestDistinct(t *testing.T) {
	t.Run("should remove duplicate integers", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 3, 4, 5, 5}
		seq := From(slice)

		distinct := Distinct(seq)
		result := Collect(distinct)

		assert.ElementsMatch(t, []int{1, 2, 3, 4, 5}, result)
	})

	t.Run("should remove duplicate strings", func(t *testing.T) {
		slice := []string{"a", "b", "a", "c", "b"}
		seq := From(slice)

		distinct := Distinct(seq)
		result := Collect(distinct)

		assert.ElementsMatch(t, []string{"a", "b", "c"}, result)
	})

	t.Run("should return same elements when all are unique", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		seq := From(slice)

		distinct := Distinct(seq)
		result := Collect(distinct)

		assert.Equal(t, slice, result)
	})

	t.Run("should work with empty sequence", func(t *testing.T) {
		slice := []int{}
		seq := From(slice)

		distinct := Distinct(seq)
		result := Collect(distinct)

		assert.Empty(t, result)
	})

	t.Run("should handle early termination", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 4, 4, 5}
		seq := From(slice)

		distinct := Distinct(seq)

		var result []int
		for v := range distinct {
			result = append(result, v)
			if v == 3 {
				break
			}
		}

		assert.Equal(t, []int{1, 2, 3}, result)
	})
}

func TestCollect(t *testing.T) {
	t.Run("should collect all elements", func(t *testing.T) {
		slice := []int{1, 2, 3, 4, 5}
		seq := From(slice)

		result := Collect(seq)

		assert.Equal(t, slice, result)
	})

	t.Run("should collect empty sequence", func(t *testing.T) {
		slice := []int{}
		seq := From(slice)

		result := Collect(seq)

		assert.Empty(t, result)
	})
}

func TestChainedOperations(t *testing.T) {
	t.Run("should chain Map and Distinct", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 4}
		seq := From(slice)

		result := Collect(Distinct(Map(seq, func(i int) int {
			return i * 2
		})))

		assert.ElementsMatch(t, []int{2, 4, 6, 8}, result)
	})

	t.Run("should chain Distinct and Map", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 4}
		seq := From(slice)

		distinct := Distinct(seq)
		result := Collect(Map(distinct, func(i int) string {
			return strconv.Itoa(i * 10)
		}))

		assert.ElementsMatch(t, []string{"10", "20", "30", "40"}, result)
	})

	t.Run("should chain multiple operations", func(t *testing.T) {
		slice := []int{1, 2, 2, 3, 3, 4, 5, 5}
		seq := From(slice)

		result := Collect(Distinct(Map(
			Distinct(seq),
			func(i int) int { return i * 2 },
		)))

		assert.ElementsMatch(t, []int{2, 4, 6, 8, 10}, result)
	})
}
