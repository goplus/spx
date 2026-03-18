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

// Package main implements the spxrun command for running spx 2.0 projects.
// This command is called by xgo when a project has a "run" directive in gox.mod.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/goplus/spx/v2/cmd/spxrun/runner"
)

func main() {
	// Define command line flags
	fullscreen := flag.Bool("fullscreen", false, "Run in fullscreen mode")
	windowed := flag.Bool("windowed", false, "Run in windowed mode (opposite of fullscreen)")
	width := flag.Int("width", 0, "Window width (e.g., 1280)")
	height := flag.Int("height", 0, "Window height (e.g., 720)")
	position := flag.String("position", "", "Window position (e.g., '100,100')")
	maximized := flag.Bool("maximized", false, "Start maximized")
	alwaysOnTop := flag.Bool("always-on-top", false, "Keep window always on top")
	debug := flag.Bool("debug", false, "Enable debug mode")
	version := flag.String("version", "", "spx version to use (e.g., 'v2.0.0', 'latest')")

	// Parse command line arguments with custom logic to support project path before flags.
	// Standard Go flag package stops parsing at the first non-flag argument, so we need
	// to manually extract the project path if it appears before any flags.
	//
	// Supported usage patterns:
	//   spxrun                        # runs current directory
	//   spxrun .                      # runs current directory
	//   spxrun /path/to/project       # runs specified project
	//   spxrun . --debug              # project path followed by flags
	//   spxrun /path/to/project --fullscreen --width 1920
	//
	// Usage: spxrun [project_path] [flags]
	projectPath := "."

	// Check if the first argument exists and is not a flag (doesn't start with '-').
	// If so, treat it as the project path and remove it from os.Args before flag.Parse().
	if len(os.Args) > 1 && (len(os.Args[1]) == 0 || os.Args[1][0] != '-') {
		projectPath = os.Args[1]
		// Reconstruct os.Args without the project path so flag.Parse() only sees flags.
		// We must preserve os.Args[0] (the program name) as flag.Parse() expects it.
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}

	// Ensure projectPath is not empty (should never happen due to default value,
	// but adding this as a safety check)
	if projectPath == "" {
		projectPath = "."
	}

	// Parse remaining flags
	flag.Parse()

	// Create runner with optional version from flag
	// If not specified, defaults to "latest" in runner.New()
	r, err := runner.New(projectPath, *version)
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	// Configure runtime options
	opts := &runner.RuntimeOptions{
		Fullscreen:  *fullscreen,
		Windowed:    *windowed,
		Width:       *width,
		Height:      *height,
		Position:    *position,
		Maximized:   *maximized,
		AlwaysOnTop: *alwaysOnTop,
		Debug:       *debug,
	}

	// Run the project with options
	if err := r.RunWithOptions(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
