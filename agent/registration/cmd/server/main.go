// Copyright (c) 2025 Metaform Systems, Inc
//
// This program and the accompanying materials are made available under the
// terms of the Apache License, Version 2.0 which is available at
// https://www.apache.org/licenses/LICENSE-2.0
//
// SPDX-License-Identifier: Apache-2.0
//
// Contributors:
//
//	Metaform Systems, Inc. - initial API and implementation

package main

import (
	"github.com/eclipse-cfm/cfm/agent/registration/launcher"
	"github.com/eclipse-cfm/cfm/common/runtime"
)

// The entry point for the EDC-V agent runtime.
func main() {
	launcher.LaunchAndWaitSignal(runtime.CreateSignalShutdownChan())
}
