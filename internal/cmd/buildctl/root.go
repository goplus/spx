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
	"fmt"
	"os"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/dockercmd"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/engine"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v3/internal/cmd/buildctl/tool"
	"github.com/goplus/spx/v3/internal/cmd/buildctl/workflow"
)

var (
	rootRunSetup  = runSetup
	rootRunBuild  = runBuild
	rootRunDoctor = runDoctor
)

func runOtherRootCommand(args []string) error {
	if len(args) == 0 {
		printRootUsage()
		return shared.ErrUsage
	}

	switch args[0] {
	case "setup":
		return rootRunSetup(args[1:])
	case "build":
		return rootRunBuild(args[1:])
	case "doctor":
		return rootRunDoctor(args[1:])
	case "env":
		return runEnv(args[1:])
	case "tool":
		return toolpkg.Run(args[1:])
	case "engine":
		return engine.Run(args[1:])
	case "docker":
		return dockercmd.Run(args[1:])
	case "workflow":
		return workflow.Run(args[1:])
	case "help", "-h", "--help":
		printRootUsage()
		return nil
	default:
		printRootUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printRootUsage() {
	fmt.Fprintln(os.Stderr, "Usage: buildctl <command> [options]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  setup      Prepare host, Web, or full development assets")
	fmt.Fprintln(os.Stderr, "  build      Build a complete developer-facing target")
	fmt.Fprintln(os.Stderr, "  doctor     Validate and print the resolved build configuration")
	fmt.Fprintln(os.Stderr, "  env        Print shared build environment values")
	fmt.Fprintln(os.Stderr, "  tool       Install build tooling")
	fmt.Fprintln(os.Stderr, "  engine     Download engine assets and manage the build lock")
	fmt.Fprintln(os.Stderr, "  runtime    Export runtime artifacts")
	fmt.Fprintln(os.Stderr, "  docker     Run unsupported legacy container workflows (independent toolchain)")
	fmt.Fprintln(os.Stderr, "  workflow   Run higher-level local build workflows")
}
