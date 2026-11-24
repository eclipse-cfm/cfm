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

package natsagent

import (
	"fmt"

	"github.com/eclipse-cfm/cfm/assembly/health"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/spf13/viper"
)

const (
	uriKey    = "uri"
	bucketKey = "bucket"
	streamKey = "stream"
	// namespaceKey configures the namespace of the activities the agent executes, see LauncherConfig.Namespace
	namespaceKey = "namespace"
)

type LauncherConfig struct {
	AgentName    string
	ConfigPrefix string
	// Namespace is the namespace of the activities the agent executes. The agent only receives the activity messages
	// of its namespace, and only reads and writes the state of the orchestrations of its namespace. The configuration
	// key "namespace" overrides it. When both are empty, the agent is in model.DefaultNamespace.
	Namespace        string
	ActivityType     string
	AssemblyProvider func() []system.ServiceAssembly
	NewProcessor     func(ctx *AgentContext) api.ActivityProcessor
	ServiceName      string
}

type AgentContext struct {
	Monitor  system.LogMonitor
	Registry AgentRegistry
	Config   *viper.Viper
}

type AgentRegistry interface {
	Resolve(serviceType system.ServiceType) any
	ResolveOptional(serviceType system.ServiceType) (any, bool)
}

type agentConfig struct {
	Name         string
	ClientConfig natsclient.ClientConfig
	StreamName   string
	Namespace    string
	VConfig      *viper.Viper
}

func LaunchAgent(shutdown <-chan struct{}, config LauncherConfig) {
	mode := runtime.LoadMode()

	cfg := loadAgentConfig(config.AgentName, config.ConfigPrefix, config.Namespace, mode)

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

	agentAssembly := &agentServiceAssembly{
		agentName:        config.AgentName,
		namespace:        cfg.Namespace,
		activityType:     config.ActivityType,
		clientConfig:     cfg.ClientConfig,
		streamName:       cfg.StreamName,
		newProcessor:     config.NewProcessor,
		requires:         requires,
		assemblyProvider: config.AssemblyProvider,
	}

	assembler.Register(agentAssembly)
	for _, assembly := range health.AgentAssemblies(cfg.VConfig) {
		assembler.Register(assembly)
	}

	if err := runtime.SetupTelemetry(config.ServiceName, shutdown); err != nil {
		monitor.Warnf("Error setting up telemetry: %s. Traces and metrics will not be available.", err.Error())
	}

	runtime.AssembleAndLaunch(assembler, cfg.Name, monitor, shutdown)
}

func loadAgentConfig(name string, configPrefix string, namespace string, mode system.RuntimeMode) *agentConfig {
	vConfig := system.LoadConfigOrPanic(configPrefix)
	if configured := vConfig.GetString(namespaceKey); configured != "" {
		namespace = configured
	}
	namespace = model.NamespaceOrDefault(namespace)
	if err := model.ValidateNamespace(namespace); err != nil {
		panic(fmt.Errorf("error loading agent configuration %s.%s: %w", configPrefix, namespaceKey, err))
	}
	uri := vConfig.GetString(uriKey)
	bucketValue := vConfig.GetString(bucketKey)
	streamValue := vConfig.GetString(streamKey)

	err := runtime.CheckRequiredParams(
		fmt.Sprintf("%s.%s", configPrefix, uriKey), uri,
		fmt.Sprintf("%s.%s", configPrefix, bucketKey), bucketValue,
		fmt.Sprintf("%s.%s", configPrefix, streamKey), streamValue)
	if err != nil {
		panic(fmt.Errorf("error loading agent configuration: %w", err))
	}

	auth, err := natsclient.AuthFromConfig(vConfig, mode)
	if err != nil {
		panic(fmt.Errorf("error loading agent configuration: %w", err))
	}

	jetStreamConfig, err := natsclient.JetStreamFromConfig(vConfig)
	if err != nil {
		panic(fmt.Errorf("error loading agent configuration: %w", err))
	}

	return &agentConfig{
		Name:         name,
		ClientConfig: natsclient.ClientConfig{URL: uri, Bucket: bucketValue, Auth: auth, TLS: natsclient.TLSFromConfig(vConfig), Resources: jetStreamConfig},
		StreamName:   streamValue,
		Namespace:    namespace,
		VConfig:      vConfig,
	}
}
