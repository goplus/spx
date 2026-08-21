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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (r CommandRunner) RunScript(relativePath string, args ...string) error {
	scriptPath := filepath.Join(r.RepoRoot, relativePath)
	cmdArgs := append([]string{scriptPath}, args...)
	env, err := buildctlCommandEnv()
	if err != nil {
		return fmt.Errorf("resolve script environment: %w", err)
	}
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = r.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = envMapToSlice(env)
	return cmd.Run()
}
