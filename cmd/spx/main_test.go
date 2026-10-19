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
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestInstallScriptUsesExistingGenerateTarget(t *testing.T) {
	content, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}

	if strings.Contains(string(content), "internal/gengo/embedded_pkgs.go") {
		t.Fatal("install script should not reference removed internal/gengo generate target")
	}
	if strings.Contains(string(content), "go generate ") {
		t.Fatal("install script should not invoke go generate during install")
	}
}

func TestMainCommand(t *testing.T) {
	if encoded := os.Getenv("SPX_CLI_TEST_ARGS"); encoded != "" {
		var args []string
		if err := json.Unmarshal([]byte(encoded), &args); err != nil {
			t.Fatal(err)
		}
		os.Args = append([]string{"spx"}, args...)
		flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
		main()
		return
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
		code int
	}{
		{"help", []string{"help"}, "Available commands:", 0},
		{"unknown command", []string{"missing-command"}, "unknown command: missing-command", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestMainCommand$")
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), "SPX_CLI_TEST_ARGS="+string(encoded))
			// Include the CLI subprocess in the parent test's coverage report.
			if f := flag.Lookup("test.gocoverdir"); f != nil && f.Value.String() != "" {
				dir := f.Value.String()
				cmd.Args = append(cmd.Args, "-test.gocoverdir="+dir)
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+dir)
			}
			output, err := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tt.code {
				t.Fatalf("CLI exit = %v, error %v; want %d\n%s", cmd.ProcessState, err, tt.code, output)
			}
			if !strings.Contains(string(output), tt.want) {
				t.Fatalf("CLI output = %q, want %q", output, tt.want)
			}
		})
	}
}
