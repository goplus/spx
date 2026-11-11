/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package command

import (
	"flag"
	"slices"
	"strings"
	"testing"
)

func TestInitializeFlagsDefaults(t *testing.T) {
	setTestCommandLine(t, "spx", "run")
	cmd := &CmdTool{Args: ExtraArgs{
		Path:    "old path",
		Verbose: true,
	}}
	help := cmd.initializeFlags()
	if *help {
		t.Fatal("help defaults to true")
	}
	defaults := map[string]any{
		"h": false, "serveraddr": "", "path": ".", "controller": "",
		"servermode": false, "headless": false, "arch": "", "onlys": false,
		"onlyc": false, "tags": "", "target": "esp32", "nomap": false,
		"install": false, "debugweb": false, "fullscreen": false,
		"build": "normal", "mode": "none", "movie": false, "v": false, "o": "",
	}
	count := 0
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		count++
		want, ok := defaults[f.Name]
		if !ok {
			t.Errorf("unexpected flag %q", f.Name)
			return
		}
		if got := f.Value.(flag.Getter).Get(); got != want {
			t.Errorf("%s default = %#v, want %#v", f.Name, got, want)
		}
		if got := f.Value.String(); got != f.DefValue {
			t.Errorf("%s help default = %q, want %q", f.Name, f.DefValue, got)
		}
		_, wantBool := want.(bool)
		boolFlag, isBool := f.Value.(interface{ IsBoolFlag() bool })
		if isBool != wantBool || isBool && !boolFlag.IsBoolFlag() {
			t.Errorf("%s boolean flag behavior changed", f.Name)
		}
	})
	if count != len(defaults) {
		t.Fatalf("registered %d flags, want %d", count, len(defaults))
	}
	if cmd.Args.Path != "." || cmd.Args.Verbose {
		t.Fatalf("initialization retained old arguments: %+v", cmd.Args)
	}
	if err := validateBuildLauncherArgs(cmd.Args); err != nil {
		t.Fatalf("default launcher flags: %v", err)
	}
}

func TestParseFlagsPreservesExplicitValues(t *testing.T) {
	setTestCommandLine(t, "spx", "run",
		"-path=", "-serveraddr=host:42", "-controller=Player", "-arch=arm64",
		"-tags=feature", "-target=", "-build=", "-mode=", "-o=out file",
		"-servermode", "-headless", "-onlys", "-onlyc", "-nomap", "-install",
		"-debugweb", "-fullscreen", "-movie", "-v", "-v=false",
		"positional", "-path=not-a-flag")
	cmd := &CmdTool{}
	if err := cmd.parseCommandLineArgs(cmd.initializeFlags()); err != nil {
		t.Fatal(err)
	}
	want := ExtraArgs{
		CmdName: "run", Path: "", ServerAddr: "host:42",
		ServerMode: true, ControllerName: "Player",
		HeadlessMode: true, Arch: "arm64",
		OnlyServer: true, OnlyClient: true, Tags: "feature",
		Target: "", NoMap: true, Install: true,
		DebugWebService: true, FullScreen: true,
		Build: "", Mode: "", Movie: true,
		Verbose: false, Output: "out file",
	}
	if cmd.Args != want {
		t.Fatalf("parsed arguments = %+v, want %+v", cmd.Args, want)
	}
	if got := flag.Args(); !slices.Equal(got, []string{"positional", "-path=not-a-flag"}) {
		t.Fatalf("remaining arguments = %q", got)
	}
	if got := cmd.Args.String(); !slices.Equal(got, []string{
		"-serveraddr", "host:42", "-servermode", "-controller", "Player",
		"--headless", "--nomap", "--debugweb", "--fullscreen",
	}) {
		t.Fatalf("runtime arguments = %q", got)
	}
}

func TestLauncherExplicitEmptyDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PATH", t.TempDir())
	setTestCommandLine(t, "spx", "buildlauncher", "-path=", "-build=", "-mode=", "-target=", "-v=false")
	cmd := &CmdTool{}
	if err := cmd.parseCommandLineArgs(cmd.initializeFlags()); err != nil {
		t.Fatal(err)
	}
	if err := validateBuildLauncherArgs(cmd.Args); err != nil {
		t.Fatalf("empty launcher arguments: %v", err)
	}
	if got := buildLauncherBuildFlags(cmd.Args); got != nil {
		t.Fatalf("empty launcher build flags = %q, want nil", got)
	}
	if err := cmd.runBuildLauncher(); err == nil || !strings.HasPrefix(err.Error(), "buildlauncher: Go command:") {
		t.Fatalf("empty launcher flags stopped before resolving Go: %v", err)
	}
}

func TestExtraArgsZeroValue(t *testing.T) {
	cmd := &CmdTool{}
	if got := cmd.Args.String(); got != nil {
		t.Fatalf("zero runtime arguments = %q, want nil", got)
	}
	if got := cmd.SafeTagArgs(); got != "" {
		t.Fatalf("zero tags = %q", got)
	}
	if err := validateBuildLauncherArgs(cmd.Args); err != nil {
		t.Fatalf("zero launcher arguments: %v", err)
	}
}
