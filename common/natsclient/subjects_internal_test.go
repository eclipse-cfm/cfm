//  Copyright (c) 2026 Metaform Systems, Inc
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubjectCovers(t *testing.T) {
	cases := []struct {
		super, sub string
		want       bool
	}{
		{"events.>", "events.contract.definition.created", true},
		{"events.>", "events.contract.definition.>", true},
		{"events.contract.definition.>", "events.contract.definition.created", true},
		{"events.contract.definition.created", "events.contract.definition.created", true},
		{"events.*", "events.created", true},
		{"events.*", "events.contract.created", false}, // "*" is one token only
		{"events.*", "events.>", false},                // ">" may expand to several tokens
		{"events.asset.>", "events.contract.definition.>", false},
		{"events.contract.definition.created", "events.contract.definition.>", false}, // sub is broader
		{"events.contract.definition.created", "events.contract.definition.updated", false},
		{"events.contract", "events.contract.definition", false}, // super needs fewer tokens
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, subjectCovers(c.super, c.sub), "subjectCovers(%q, %q)", c.super, c.sub)
	}
}

func TestSubjectsCollide(t *testing.T) {
	assert.True(t, subjectsCollide("a.*", "*.b"))      // a.b matches both
	assert.True(t, subjectsCollide("events.>", "events.contract.created"))
	assert.False(t, subjectsCollide("events.asset.created", "events.contract.created"))
	assert.False(t, subjectsCollide("events.a", "events.a.b")) // differing lengths, no ">"
}

func TestMergeStreamSubjects(t *testing.T) {
	t.Run("subject already covered is skipped", func(t *testing.T) {
		merged, changed, err := mergeStreamSubjects([]string{"events.>"}, []string{"events.contract.definition.created"})
		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, []string{"events.>"}, merged)
	})

	t.Run("broader subject supersedes narrower ones", func(t *testing.T) {
		merged, changed, err := mergeStreamSubjects(
			[]string{"events.contract.definition.created", "events.contract.definition.updated"},
			[]string{"events.contract.definition.>"})
		require.NoError(t, err)
		assert.True(t, changed)
		assert.Equal(t, []string{"events.contract.definition.>"}, merged)
	})

	t.Run("disjoint subject is appended", func(t *testing.T) {
		merged, changed, err := mergeStreamSubjects([]string{"events.contract.definition.created"}, []string{"events.asset.created"})
		require.NoError(t, err)
		assert.True(t, changed)
		assert.ElementsMatch(t, []string{"events.contract.definition.created", "events.asset.created"}, merged)
	})

	t.Run("identical subject is a no-op", func(t *testing.T) {
		_, changed, err := mergeStreamSubjects([]string{"events.contract.definition.>"}, []string{"events.contract.definition.>"})
		require.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("partial overlap is rejected", func(t *testing.T) {
		_, _, err := mergeStreamSubjects([]string{"a.*"}, []string{"*.b"})
		assert.Error(t, err)
	})
}
