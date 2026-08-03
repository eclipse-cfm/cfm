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

package fixtures

import (
	"net"
	"testing"

	"github.com/eclipse-cfm/cfm/common/system"
)

// IsolateConfig points the configuration file search path at an empty temp directory so that configuration files on
// the developer's machine (e.g. $HOME/.appname) cannot leak into the test. The override is undone when the test ends.
func IsolateConfig(t *testing.T) {
	t.Setenv(system.ConfigDirEnvVar, t.TempDir())
}

// GetRandomPort returns a random available port or fails the test
func GetRandomPort(t *testing.T) int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	//goland:noinspection GoUnhandledErrorResult
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}
