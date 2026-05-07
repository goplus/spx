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

// Package main implements the spxrunner command for running SPX PC projects.
// This is the command-package entry used by xgo's generic runner directive.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/goplus/spx/v2/cmd/spxrunner/runner"
)

func main() {
	fullscreen := flag.Bool("fullscreen", false, "Run in fullscreen mode")
	windowed := flag.Bool("windowed", false, "Run in windowed mode (opposite of fullscreen)")
	width := flag.Int("width", 0, "Window width (e.g., 1280)")
	height := flag.Int("height", 0, "Window height (e.g., 720)")
	position := flag.String("position", "", "Window position (e.g., '100,100')")
	maximized := flag.Bool("maximized", false, "Start maximized")
	alwaysOnTop := flag.Bool("always-on-top", false, "Keep window always on top")
	debug := flag.Bool("debug", false, "Enable debug mode")
	version := flag.String("version", "", "SPX version to use (e.g., 'v2.0.0', 'latest')")

	projectPath := "."
	if len(os.Args) > 1 && (len(os.Args[1]) == 0 || os.Args[1][0] != '-') {
		projectPath = os.Args[1]
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}
	if projectPath == "" {
		projectPath = "."
	}

	flag.Parse()

	r, err := runner.New(projectPath, *version)
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

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

	if err := r.RunWithOptions(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
