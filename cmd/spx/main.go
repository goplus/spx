/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/goplus/spx/v2/cmd/spx/internal/command"
	"github.com/goplus/spx/v2/internal/release"
	"github.com/goplus/spx/v2/internal/scaffold"
)

var (
	//go:embed template/platform/*
	platformFS embed.FS

	//go:embed template/project/*
	projectFS embed.FS

	//go:embed appname.txt
	appName string
)

func main() {
	cmd := &command.CmdTool{}

	// Initialize with default values
	cmd.ServerPort = 8005
	cmd.ProjectRelPath = "/project"
	cmd.BinPostfix = ""

	// Initialize with provided values
	cmd.ProjectFS = projectFS
	cmd.PlatformFS = platformFS
	cmd.Version = release.DefaultReleaseMeta().Runtime.Version
	cmd.GoModTemplate = scaffold.GoMod()

	// Initialize the Args field if not already initialized
	err := cmd.RunCmd(appName, appName, cmd.Version, projectFS, "template/project", "project")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to run cmd:", err)
		os.Exit(1)
	}
}
