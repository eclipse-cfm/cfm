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

package launcher

import (
	"fmt"

	"github.com/eclipse-cfm/cfm/assembly/routing"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/telemetry"
	"github.com/eclipse-cfm/cfm/tmanager/core"
	"github.com/eclipse-cfm/cfm/tmanager/handler"
	"github.com/eclipse-cfm/cfm/tmanager/memorystore"
	"github.com/eclipse-cfm/cfm/tmanager/natsprovision"
	"github.com/eclipse-cfm/cfm/tmanager/sqlstore"
)

const (
	logPrefix    = "tmanager"
	defaultPort  = 8080
	configPrefix = "tm"
	key          = "httpPort"

	postgresKey = "postgres"

	uriKey    = "uri"
	bucketKey = "bucket"
	streamKey = "stream"
)

func LaunchAndWaitSignal() {
	Launch(runtime.CreateSignalShutdownChan())
}

func Launch(shutdown <-chan struct{}) {
	mode := runtime.LoadMode()

	logMonitor := runtime.LoadLogMonitor(logPrefix, mode)
	//goland:noinspection GoUnhandledErrorResult
	defer logMonitor.Sync()

	vConfig := system.LoadConfigOrPanic(configPrefix)
	vConfig.SetDefault(key, defaultPort)

	uri := vConfig.GetString(uriKey)
	bucketValue := vConfig.GetString(bucketKey)
	streamValue := vConfig.GetString(streamKey)

	err := runtime.CheckRequiredParams(
		fmt.Sprintf("%s.%s", configPrefix, uriKey), uri,
		fmt.Sprintf("%s.%s", configPrefix, bucketKey), bucketValue,
		fmt.Sprintf("%s.%s", configPrefix, streamKey), streamValue)
	if err != nil {
		panic(fmt.Errorf("error launching Tenant Manager: %w", err))
	}

	assembler := system.NewServiceAssembler(logMonitor, vConfig, mode)
	assembler.Register(&routing.RouterServiceAssembly{})
	assembler.Register(&handler.HandlerServiceAssembly{})
	assembler.Register(&core.TMCoreServiceAssembly{})
	assembler.Register(telemetry.NewTelemetryServiceAssembly(logPrefix))

	if vConfig.IsSet(postgresKey) {
		assembler.Register(&sqlstore.PostgresServiceAssembly{})
	} else {
		assembler.Register(&store.NoOpTrxAssembly{})
		assembler.Register(&memorystore.InMemoryServiceAssembly{})
	}

	assembler.Register(natsprovision.NewNatsOrchestrationServiceAssembly(uri, bucketValue, streamValue))

	runtime.AssembleAndLaunch(assembler, "Tenant Manager", logMonitor, shutdown)
}
