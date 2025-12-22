/*
 * Copyright (c) 2024 The XGo Authors (xgo.dev). All rights reserved.
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

// Package main implements the spxrun command for running SPX 2.0 projects.
// This command is called by xgo when a project has a "run" directive in gop.mod.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/goplus/spx/v2/cmd/spxrun/runner"
)

func main() {
	// Parse command line flags
	path := flag.String("path", ".", "project path")
	flag.Parse()

	// Create runner
	r, err := runner.New(*path)
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	// Run the project
	if err := r.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
