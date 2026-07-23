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

package util

import (
	"os"
	"os/exec"

	"github.com/goplus/spx/v3/cmd/spx/internal/logutil"
)

type CommandOptions struct {
	Env []string
	Dir string
}

// RunCommandInDir runs a command in dir and exits the process on failure.
func RunCommandInDir(dir string, name string, args ...string) error {
	return RunCommand(nil, dir, name, args...)
}

// RunCommandWithEnv runs a command with envVars and exits the process on failure.
func RunCommandWithEnv(envVars []string, name string, args ...string) error {
	return RunCommand(envVars, "", name, args...)
}

// RunCommand runs a command and terminates the process if it fails.
func RunCommand(envVars []string, dir string, name string, args ...string) error {
	if err := ExecCommand(CommandOptions{Env: envVars, Dir: dir}, name, args...); err != nil {
		logutil.Fatalf("command %s failed: %v", name, err)
	}
	return nil
}

// ExecCommand runs a command and returns any execution error to the caller.
func ExecCommand(options CommandOptions, name string, args ...string) error {
	execCmd := exec.Command(name, args...)
	applyCommandOptions(execCmd, options)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

// OutputCommand runs a command and returns its stdout without exiting the process.
func OutputCommand(options CommandOptions, name string, args ...string) ([]byte, error) {
	execCmd := exec.Command(name, args...)
	applyCommandOptions(execCmd, options)
	return execCmd.Output()
}

func applyCommandOptions(execCmd *exec.Cmd, options CommandOptions) {
	if len(options.Env) > 0 {
		execCmd.Env = append(os.Environ(), options.Env...)
	}
	if options.Dir != "" {
		execCmd.Dir = options.Dir
	}
}

func RunXGo(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "xgo", args...)
}

func RunGolang(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "go", args...)
}

// RunTinyGo runs tinygo.
func RunTinyGo(envVars []string, args ...string) error {
	return RunCommandWithEnv(envVars, "tinygo", args...)
}
