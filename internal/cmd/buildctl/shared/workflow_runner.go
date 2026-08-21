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
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
)

func (r CommandRunner) ListDemoDirs() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(r.RepoRoot, "tutorial"))
	if err != nil {
		return nil, err
	}

	demos := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			demos = append(demos, filepath.Join("tutorial", entry.Name()))
		}
	}
	sort.Strings(demos)
	return demos, nil
}

func (r CommandRunner) StopWebServers() error {
	if runtime.GOOS == "windows" || os.Getenv("OS") == "Windows_NT" {
		_ = exec.Command("taskkill", "/F", "/FI", "IMAGENAME eq python.exe").Run()
		_ = exec.Command("taskkill", "/F", "/FI", "IMAGENAME eq python3.exe").Run()
		return nil
	}

	cmd := exec.Command("pgrep", "-f", "gdspx_web_server.py")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return err
	}

	for _, field := range bytes.Fields(output) {
		pid, err := strconv.Atoi(string(field))
		if err != nil {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		_ = process.Kill()
	}
	return nil
}
