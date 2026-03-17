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

package system

import (
	"fmt"
	"strings"

	"github.com/eclipse-cfm/cfm/common/dag"
	"github.com/spf13/viper"
)

const (
	DebugMode       = "debug"
	DevelopmentMode = "development"
	ProductionMode  = "production"
)

// ServiceType is used to specify a key for a service in the ServiceRegistry.
type ServiceType string

// ServiceRegistry manages a collection of service instances that can be resolved by dependent systems.
type ServiceRegistry struct {
	services map[ServiceType]any
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[ServiceType]any),
	}
}

// Register registers a service instance by type.
func (r *ServiceRegistry) Register(serviceType ServiceType, service any) {
	r.services[serviceType] = service
}

// Resolve retrieves a service instance by its type, returning the service or panicing if it does not exist.
func (r *ServiceRegistry) Resolve(serviceType ServiceType) any {
	if service, exists := r.services[serviceType]; exists {
		return service
	}
	panic(fmt.Errorf("service not found: %s", serviceType))
}

// ResolveOptional retrieves a service instance by its type, returning the service and a boolean indicating its existence.
func (r *ServiceRegistry) ResolveOptional(serviceType ServiceType) (any, bool) {
	if service, exists := r.services[serviceType]; exists {
		return service, true
	}
	return nil, false
}

type RuntimeMode string

func (m RuntimeMode) IsValid() bool {
	switch m {
	case DebugMode, DevelopmentMode, ProductionMode:
		return true
	}
	return false
}

func ParseRuntimeMode(mode string) (RuntimeMode, error) {
	switch strings.ToLower(mode) {
	case "production", "prod":
		return ProductionMode, nil
	case "development", "dev":
		return DevelopmentMode, nil
	case "debug":
		return DebugMode, nil
	default:
		return "", fmt.Errorf("invalid runtime mode: %s", mode)
	}
}

// ServiceAssembly is a subsystem that contributes services to a runtime.
// The assembly provides zero or more services that may be resolved by other assemblies and requires 0 or more services.
type ServiceAssembly interface {
	Name() string
	Provides() []ServiceType
	Requires() []ServiceType
	Init(*InitContext) error
	Prepare(*InitContext) error
	Start(*StartContext) error
	Finalize() error
	Shutdown() error
}

type InitContext struct {
	StartContext
}

type StartContext struct {
	Registry   *ServiceRegistry
	LogMonitor LogMonitor
	Config     *viper.Viper
	Mode       RuntimeMode
}

// GetConfigIntOrDefault retrieves an integer config value by key or returns the provided defaultValue if the key is not set.
func (c InitContext) GetConfigIntOrDefault(key string, defaultValue int) int {
	if !c.Config.IsSet(key) {
		return defaultValue
	}
	return c.Config.GetInt(key)
}

// GetConfigStrOrDefault retrieves a string config value by key or returns the provided defaultValue if the key is not set.
func (c InitContext) GetConfigStrOrDefault(key string, defaultValue string) string {
	if !c.Config.IsSet(key) {
		return defaultValue
	}
	return c.Config.GetString(key)
}

// ServiceAssembler manages the registration, dependency resolution, and initialization of service assemblies in a runtime.
type ServiceAssembler struct {
	assemblies []ServiceAssembly
	monitor    LogMonitor
	mode       RuntimeMode
	vConfig    *viper.Viper
	registry   *ServiceRegistry
}

func NewServiceAssembler(monitor LogMonitor, vConfig *viper.Viper, mode RuntimeMode) *ServiceAssembler {
	return &ServiceAssembler{
		assemblies: make([]ServiceAssembly, 0),
		monitor:    monitor,
		mode:       mode,
		vConfig:    vConfig,
		registry:   NewServiceRegistry(),
	}
}

func (a *ServiceAssembler) Resolve(serviceType ServiceType) any {
	return a.registry.Resolve(serviceType)
}

func (a *ServiceAssembler) ResolveOptional(serviceType ServiceType) (any, bool) {
	return a.registry.ResolveOptional(serviceType)
}

func (a *ServiceAssembler) Register(assembly ServiceAssembly) {
	a.assemblies = append(a.assemblies, assembly)
}

func (a *ServiceAssembler) Assemble() error {
	assemblyGraph := dag.NewGraph[ServiceAssembly]()
	mappedAssemblies := make(map[ServiceType]ServiceAssembly)
	for _, assembly := range a.assemblies {
		// use a new variable in the loop to avoid pointer issues
		assembly := assembly
		assemblyGraph.AddVertex(assembly.Name(), &assembly)
		for _, provided := range assembly.Provides() {
			mappedAssemblies[provided] = assembly
		}
	}
	for _, assembly := range a.assemblies {
		for _, required := range assembly.Requires() {
			requiredService, exists := mappedAssemblies[required]
			if !exists {
				return fmt.Errorf("required assembly not found for assembly %s: %s", assembly.Name(), required)
			}
			assemblyGraph.AddEdge(assembly.Name(), requiredService.Name())
		}
	}
	sorted := assemblyGraph.TopologicalSort()
	if sorted.HasCycle {
		return fmt.Errorf("cycle detected in assembly graph")
	}

	reverseOrder := make([]*dag.Vertex[ServiceAssembly], len(sorted.SortedOrder))
	copy(reverseOrder, sorted.SortedOrder)
	reverse(reverseOrder)

	startCtx := &StartContext{
		Registry:   a.registry,
		LogMonitor: a.monitor,
		Config:     a.vConfig,
		Mode:       a.mode,
	}
	initCtx := &InitContext{
		StartContext: *startCtx,
	}

	for _, v := range reverseOrder {
		e := v.Value.Init(initCtx)
		a.monitor.Debugf("Initialized: " + v.Value.Name())
		if e != nil {
			return fmt.Errorf("error initializing assembly %s: %w", v.Value.Name(), e)
		}
	}

	for _, v := range reverseOrder {
		e := v.Value.Prepare(initCtx)
		a.monitor.Debugf("Prepared: " + v.Value.Name())
		if e != nil {
			return fmt.Errorf("error preparing assembly %s: %w", v.Value.Name(), e)
		}
	}

	for _, v := range reverseOrder {
		e := v.Value.Start(startCtx)
		a.monitor.Debugf("Started: " + v.Value.Name())
		if e != nil {
			return fmt.Errorf("error starting assembly %s: %w", v.Value.Name(), e)
		}
	}

	a.assemblies = mapToAssemblies(reverseOrder)

	return nil
}

func (a *ServiceAssembler) Shutdown() error {
	for _, v := range a.assemblies {
		e := v.Finalize()
		a.monitor.Debugf("Finalized: " + v.Name())
		if e != nil {
			return fmt.Errorf("error finalizing assembly %s: %w", v.Name(), e)
		}
	}
	for _, v := range a.assemblies {
		e := v.Shutdown()
		a.monitor.Debugf("Shutdown: " + v.Name())
		if e != nil {
			return fmt.Errorf("error shutting down assembly %s: %w", v.Name(), e)
		}
	}
	return nil
}

func mapToAssemblies(sorted []*dag.Vertex[ServiceAssembly]) []ServiceAssembly {
	result := make([]ServiceAssembly, len(sorted))
	for i, vertex := range sorted {
		result[i] = vertex.Value
	}
	return result
}

func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

type DefaultServiceAssembly struct {
}

func (d *DefaultServiceAssembly) Provides() []ServiceType {
	return []ServiceType{}
}

func (d *DefaultServiceAssembly) Requires() []ServiceType {
	return []ServiceType{}
}

func (d *DefaultServiceAssembly) Init(*InitContext) error {
	return nil
}

func (d *DefaultServiceAssembly) Prepare(*InitContext) error {
	return nil
}

func (d *DefaultServiceAssembly) Start(*StartContext) error {
	return nil
}

func (d *DefaultServiceAssembly) Finalize() error {
	return nil
}

func (d *DefaultServiceAssembly) Shutdown() error {
	return nil
}
