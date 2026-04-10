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

	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
	"github.com/spf13/viper"
)

const (
	uriKey    = "uri"
	bucketKey = "bucket"
	streamKey = "stream"
)

type LauncherConfig struct {
	AgentName        string
	ConfigPrefix     string
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
	Name       string
	URI        string
	Bucket     string
	StreamName string
	VConfig    *viper.Viper
}

func LaunchAgent(shutdown <-chan struct{}, config LauncherConfig) {
	cfg := loadAgentConfig(config.AgentName, config.ConfigPrefix)

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

	agentAssembly := &agentServiceAssembly{
		agentName:        config.AgentName,
		activityType:     config.ActivityType,
		uri:              cfg.URI,
		bucket:           cfg.Bucket,
		streamName:       cfg.StreamName,
		newProcessor:     config.NewProcessor,
		requires:         requires,
		assemblyProvider: config.AssemblyProvider,
	}

	assembler.Register(agentAssembly)

	if err := runtime.SetupTelemetry(config.ServiceName, shutdown); err != nil {
		monitor.Warnf("Error setting up telemetry: %s. Traces and metrics will not be available.", err.Error())
	}

	runtime.AssembleAndLaunch(assembler, cfg.Name, monitor, shutdown)
}

func loadAgentConfig(name string, configPrefix string) *agentConfig {
	vConfig := system.LoadConfigOrPanic(configPrefix)
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
	return &agentConfig{
		Name:       name,
		URI:        uri,
		Bucket:     bucketValue,
		StreamName: streamValue,
		VConfig:    vConfig,
	}
}
