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

// Package lifecycleagent provides a generic framework for standalone agents that are triggered by arbitrary NATS
// JetStream messages on configurable subjects (for example "events.contract.definition.created"), as opposed to the
// orchestration-specific agents in pmanager/natsagent which only react to orchestration activity messages.
package lifecycleagent

import (
	"context"
	"fmt"

	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

const (
	uriKey      = "uri"
	bucketKey   = "bucket"
	streamKey   = "stream"
	subjectsKey = "subjects"
	subjectKey  = "subject"
)

// EventContext carries a single decoded event together with the metadata of the NATS message that delivered it. The
// Subject and Header fields let processors that subscribe to multiple subjects branch on which event was received.
type EventContext[T any] struct {
	// Payload is the decoded event body.
	Payload T
	// Subject is the NATS subject the message was delivered on.
	Subject string
	// Header holds the NATS message headers.
	Header nats.Header
	// Raw is the undecoded message body.
	Raw []byte
}

// EventProcessor processes events delivered to a lifecycle agent. Returning a recoverable error (see
// common/types.NewRecoverableError) causes the message to be redelivered; any other error is treated as fatal and the
// message is acknowledged (dropped).
type EventProcessor[T any] interface {
	Process(ctx context.Context, evt EventContext[T]) error
}

// AgentContext provides processors with access to the runtime monitor, service registry and configuration.
type AgentContext struct {
	Monitor  system.LogMonitor
	Registry AgentRegistry
	Config   *viper.Viper
}

// AgentRegistry resolves services registered by the agent's assemblies.
type AgentRegistry interface {
	Resolve(serviceType system.ServiceType) any
	ResolveOptional(serviceType system.ServiceType) (any, bool)
}

// LauncherConfig configures a lifecycle agent.
type LauncherConfig[T any] struct {
	// AgentName is a human-readable name; it also seeds the durable consumer name.
	AgentName string
	// ServiceName is the name reported to telemetry.
	ServiceName string
	// ConfigPrefix is the viper prefix under which configuration is loaded.
	ConfigPrefix string
	// Subjects are the default subjects to subscribe to. They are merged with the `subjects`/`subject` configuration
	// values; at least one subject must result from the merge.
	Subjects []string
	// AssemblyProvider optionally provides additional service assemblies (for example HTTP clients).
	AssemblyProvider func() []system.ServiceAssembly
	// NewProcessor constructs the event processor once the agent's dependencies have been assembled.
	NewProcessor func(ctx *AgentContext) EventProcessor[T]
}

type agentConfig struct {
	Name       string
	URI        string
	Bucket     string
	StreamName string
	Subjects   []string
	VConfig    *viper.Viper
}

// LaunchAgent bootstraps and runs a lifecycle agent until the shutdown channel is closed.
func LaunchAgent[T any](shutdown <-chan struct{}, config LauncherConfig[T]) {
	cfg := loadAgentConfig(config.AgentName, config.ConfigPrefix, config.Subjects)

	mode := runtime.LoadMode()

	monitor := runtime.LoadLogMonitor(config.ConfigPrefix, mode)
	//goland:noinspection GoUnhandledErrorResult
	defer monitor.Sync()

	requires := make([]system.ServiceType, 0)
	assembler := system.NewServiceAssembler(monitor, cfg.VConfig, mode)
	if config.AssemblyProvider != nil {
		assemblies := config.AssemblyProvider()
		for _, assembly := range assemblies {
			for _, name := range assembly.Provides() {
				requires = append(requires, name)
			}
			assembler.Register(assembly)
		}
	}

	agentAssembly := &eventAgentServiceAssembly[T]{
		agentName:    config.AgentName,
		uri:          cfg.URI,
		bucket:       cfg.Bucket,
		streamName:   cfg.StreamName,
		subjects:     cfg.Subjects,
		newProcessor: config.NewProcessor,
		requires:     requires,
	}

	assembler.Register(agentAssembly)

	if err := runtime.SetupTelemetry(config.ServiceName, shutdown); err != nil {
		monitor.Warnf("Error setting up telemetry: %s. Traces and metrics will not be available.", err.Error())
	}

	runtime.AssembleAndLaunch(assembler, cfg.Name, monitor, shutdown)
}

func loadAgentConfig(name string, configPrefix string, defaultSubjects []string) *agentConfig {
	vConfig := system.LoadConfigOrPanic(configPrefix)
	uri := vConfig.GetString(uriKey)
	bucketValue := vConfig.GetString(bucketKey)
	streamValue := vConfig.GetString(streamKey)

	subjects := mergeSubjects(defaultSubjects, vConfig.GetStringSlice(subjectsKey), vConfig.GetString(subjectKey))

	err := runtime.CheckRequiredParams(
		fmt.Sprintf("%s.%s", configPrefix, uriKey), uri,
		fmt.Sprintf("%s.%s", configPrefix, bucketKey), bucketValue,
		fmt.Sprintf("%s.%s", configPrefix, streamKey), streamValue)
	if err != nil {
		panic(fmt.Errorf("error loading agent configuration: %w", err))
	}
	if len(subjects) == 0 {
		panic(fmt.Errorf("error loading agent configuration: at least one subject must be configured via LauncherConfig.Subjects, %s.%s or %s.%s", configPrefix, subjectsKey, configPrefix, subjectKey))
	}

	return &agentConfig{
		Name:       name,
		URI:        uri,
		Bucket:     bucketValue,
		StreamName: streamValue,
		Subjects:   subjects,
		VConfig:    vConfig,
	}
}

// mergeSubjects combines the default subjects with the configured subject slice and single-subject sugar, removing
// duplicates while preserving order.
func mergeSubjects(defaults []string, configured []string, single string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(defaults)+len(configured)+1)
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		result = append(result, s)
	}
	for _, s := range defaults {
		add(s)
	}
	for _, s := range configured {
		add(s)
	}
	add(single)
	return result
}
