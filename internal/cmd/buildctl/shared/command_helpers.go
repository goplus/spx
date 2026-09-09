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

package shared

import (
	"os"
	"os/exec"
)

// RunCommandOutputWithEnv returns combined output using the given environment
// and working directory. An empty workdir inherits the current directory.
func RunCommandOutputWithEnv(workdir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	if workdir != "" {
		cmd.Dir = workdir
	}
	return cmd.CombinedOutput()
}

func runCommandOutput(name string, args ...string) ([]byte, error) {
	return RunCommandOutputWithEnv("", os.Environ(), name, args...)
}

func runStreamingCommand(workdir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
