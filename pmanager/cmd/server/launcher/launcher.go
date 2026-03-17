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

	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/routing"
	"github.com/eclipse-cfm/cfm/common/runtime"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/core"
	"github.com/eclipse-cfm/cfm/pmanager/handler"
	"github.com/eclipse-cfm/cfm/pmanager/memorystore"
	"github.com/eclipse-cfm/cfm/pmanager/natsorchestration"
	"github.com/eclipse-cfm/cfm/pmanager/natsprovision"
	"github.com/eclipse-cfm/cfm/pmanager/sqlstore"
)

const (
	logPrefix    = "pmanager"
	defaultPort  = 8181
	configPrefix = "pm"
	httpKey      = "httpPort"
	postgresKey  = "postgres"
	uriKey       = "uri"
	bucketKey    = "bucket"
	streamKey    = "stream"
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
	vConfig.SetDefault(httpKey, defaultPort)

	uri := vConfig.GetString(uriKey)
	bucketValue := vConfig.GetString(bucketKey)
	streamValue := vConfig.GetString(streamKey)

	err := runtime.CheckRequiredParams(
		fmt.Sprintf("%s.%s", configPrefix, uriKey), uri,
		fmt.Sprintf("%s.%s", configPrefix, bucketKey), bucketValue,
		fmt.Sprintf("%s.%s", configPrefix, streamKey), streamValue)
	if err != nil {
		panic(fmt.Errorf("error launching Provision Manager: %w", err))
	}

	assembler := system.NewServiceAssembler(logMonitor, vConfig, mode)

	assembler.Register(&routing.RouterServiceAssembly{})
	assembler.Register(&handler.HandlerServiceAssembly{})

	if vConfig.IsSet(postgresKey) {
		assembler.Register(&sqlstore.PostgresServiceAssembly{})
	} else {
		assembler.Register(&store.NoOpTrxAssembly{})
		assembler.Register(&memorystore.MemoryStoreServiceAssembly{})
	}

	assembler.Register(natsorchestration.NewOrchestratorServiceAssembly(uri, bucketValue, streamValue))
	assembler.Register(natsprovision.NewProvisionServiceAssembly(streamValue))
	assembler.Register(&core.PMCoreServiceAssembly{})

	runtime.AssembleAndLaunch(assembler, "Provision Manager", logMonitor, shutdown)
}
