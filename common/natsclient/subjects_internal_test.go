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

	"github.com/eclipse-cfm/cfm/common/model"
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
	assert.True(t, subjectsCollide("a.*", "*.b")) // a.b matches both
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

func TestResponseSubject(t *testing.T) {
	assert.Equal(t, "event.default.cfm-orchestration-response", CFMOrchestrationResponseSubject)
	assert.Equal(t, CFMOrchestrationResponseSubject, ResponseSubject("", ""))
	assert.Equal(t, CFMOrchestrationResponseSubject, ResponseSubject(model.DefaultNamespace, ""))
	assert.Equal(t, "event.default.second-manager-response", ResponseSubject("", "second-manager-response"))
	assert.Equal(t, CFMOrchestrationResponseSubject, ResponseSubject("", "a.b"), "a token with a dot is not used")
	assert.Equal(t, CFMOrchestrationResponseSubject, ResponseSubject("", ">"), "a wildcard is not used")
	assert.Equal(t, CFMOrchestrationResponseSubject, ResponseSubject("", CFMOrchestration), "the manifest subject is not used")

	assert.Equal(t, "event.second.cfm-orchestration-response", ResponseSubject("second", ""))
	assert.Equal(t, "event.second.manager-response", ResponseSubject("second", "manager-response"))
	assert.Equal(t, "event.second.cfm-orchestration-response", ResponseSubject("second", "a.b"), "a token with a dot is not used")
	assert.Equal(t, "event.second.cfm-orchestration-response", ResponseSubject("second", CFMOrchestration), "the manifest subject is not used")
}

func TestSubject(t *testing.T) {
	assert.Equal(t, "event.default.test-activity", Subject("", "test-activity"))
	assert.Equal(t, "event.default.test-activity", Subject(model.DefaultNamespace, "test-activity"))
	assert.Equal(t, "event.second.test-activity", Subject("second", "test-activity"))
	assert.Equal(t, CFMOrchestrationSubject, Subject(model.DefaultNamespace, CFMOrchestration))
	assert.True(t, subjectCovers(CFMOrchestrationSubjects, Subject("second", CFMOrchestration)))
	assert.True(t, subjectCovers(CFMOrchestrationSubjects, CFMOrchestrationSubject))
	assert.False(t, subjectCovers(CFMOrchestrationSubjects, Subject("second", "test-activity")))
	assert.True(t, subjectCovers(CFMStreamSubjects, Subject("second", "test-activity")))
	assert.False(t, subjectCovers("event.second.>", Subject("third", "test-activity")))
}

func TestSubjectNamespace(t *testing.T) {
	for subject, namespace := range map[string]string{
		"event.default.test-activity": model.DefaultNamespace,
		"event.second.test-activity":  "second",
		CFMOrchestrationSubject:       model.DefaultNamespace,
		Subject("third", "anything"):  "third",
	} {
		actual, ok := SubjectNamespace(subject)
		assert.True(t, ok, subject)
		assert.Equal(t, namespace, actual, subject)
	}
	for _, subject := range []string{"other.second.test-activity", "event", "event.test-activity", "event.a.b.c", "event.$x.y"} {
		_, ok := SubjectNamespace(subject)
		assert.False(t, ok, subject)
	}
}

func TestConsumerName(t *testing.T) {
	assert.Equal(t, "default~test-activity", ConsumerName("", "test-activity"))
	assert.Equal(t, "default~test-activity", ConsumerName(model.DefaultNamespace, "test.activity"))
	assert.Equal(t, "second~test-activity", ConsumerName("second", "test.activity"))
	// the separator is not part of any namespace or token, so names of different namespaces never collide
	assert.NotEqual(t, ConsumerName("a", "b_c"), ConsumerName("a_b", "c"))
	assert.NotEqual(t, ConsumerName("a", "b-c"), ConsumerName("a-b", "c"))
	assert.NotEqual(t, CFMOrchestrationConsumer, ConsumerName(model.DefaultNamespace, CFMOrchestration))
}
