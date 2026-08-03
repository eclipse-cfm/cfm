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
	"os"
	"strings"

	"github.com/spf13/viper"
)

// ConfigDirEnvVar overrides the configuration file search path. When set, only the given directory is searched for
// configuration files, replacing the default locations entirely. This pins the config location in deployments and
// isolates tests from configuration files on the developer's machine.
const ConfigDirEnvVar = "CFM_CONFIG_DIR"

// LoadConfigOrPanic initializes a Config instance with the specified configuration name.
// Configuration will be read from a file (if it exists) and can be overridden using environment variables.
func LoadConfigOrPanic(name string) *viper.Viper {
	v := viper.New()
	v.SetConfigName(name)
	if dir := os.Getenv(ConfigDirEnvVar); dir != "" {
		v.AddConfigPath(dir)
	} else {
		v.AddConfigPath("/etc/appname/")
		v.AddConfigPath("$HOME/.appname")
		v.AddConfigPath(".")
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetEnvPrefix(name)
	err := v.ReadInConfig()

	if err != nil {
		// ignore not found error, otherwise panic
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(fmt.Errorf("error reading config file: %w", err))
		}
	}
	return v
}
