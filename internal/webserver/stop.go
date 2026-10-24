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

// Package webserver discovers and stops only identified SPX web server processes.
package webserver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const scriptName = "gdspx_web_server.py"

// Match a complete script basename, not another script with a similar name.
var scriptCommandLine = regexp.MustCompile(`(?:^|[/\\\s"'])` + regexp.QuoteMeta(scriptName) + `(?:$|[\s"'])`)

// Stopper keeps process discovery and identity checks on the same platform.
// Project callers select a recorded PID or port; StopAll selects every server.
type Stopper struct {
	goos      string
	output    func(string, ...string) ([]byte, error)
	terminate func(int) error
}

func New(goos string) Stopper {
	return Stopper{goos: goos, output: func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}, terminate: func(pid int) error {
		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return process.Kill()
	}}
}

// Stop kills a positive PID only after checking its current command line.
// Unreadable or stale processes are not killed. A failed kill is not success.
func (s Stopper) Stop(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	var output []byte
	var err error
	if s.goos == "windows" {
		query := fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"ProcessId = %d\").CommandLine", pid)
		output, err = s.powershell(query)
	} else {
		output, err = s.output("ps", "-o", "args=", "-p", strconv.Itoa(pid))
	}
	if err != nil {
		return false, nil
	}
	commandLine := string(output)
	if s.goos == "windows" {
		commandLine = strings.ToLower(commandLine)
	}
	if !scriptCommandLine.MatchString(commandLine) {
		return false, nil
	}
	if s.goos == "windows" {
		_, err = s.output("taskkill", "/F", "/PID", strconv.Itoa(pid))
	} else {
		err = s.terminate(pid)
	}
	if err != nil {
		return false, fmt.Errorf("stop web server %d: %w", pid, err)
	}
	return true, nil
}

// StopAll stops verified SPX servers, never all Python processes.
func (s Stopper) StopAll() error {
	var pids []int
	var err error
	if s.goos == "windows" {
		output, queryErr := s.powershell(fmt.Sprintf("Get-CimInstance Win32_Process -Filter \"CommandLine LIKE '%%%s%%'\" | Select-Object -ExpandProperty ProcessId", scriptName))
		pids, err = parsePIDs(strings.Fields(string(output))), queryErr
	} else {
		pids, err = s.unixPIDs("pgrep", "-f", scriptName)
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, pid := range pids {
		if _, err := s.Stop(pid); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ListeningPIDs finds candidates on a port; Stop still verifies their identity.
func (s Stopper) ListeningPIDs(port int) ([]int, error) {
	if port <= 0 {
		return nil, nil
	}
	if s.goos == "windows" {
		output, err := s.output("netstat", "-ano", "-p", "tcp")
		if err != nil {
			return nil, err
		}
		var candidates []string
		portSuffix := ":" + strconv.Itoa(port)
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") || !strings.HasSuffix(fields[1], portSuffix) {
				continue
			}
			if !strings.EqualFold(fields[3], "LISTENING") && !strings.EqualFold(fields[3], "LISTEN") {
				continue
			}
			candidates = append(candidates, fields[4])
		}
		return parsePIDs(candidates), nil
	}
	return s.unixPIDs("lsof", "-nP", "-t", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN")
}

func (s Stopper) powershell(query string) ([]byte, error) {
	return s.output("powershell", "-NoProfile", "-NonInteractive", "-Command", query)
}

// pgrep and lsof both use exit status 1 for an empty result.
func (s Stopper) unixPIDs(name string, args ...string) ([]int, error) {
	output, err := s.output(name, args...)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	return parsePIDs(strings.Fields(string(output))), nil
}

func parsePIDs(fields []string) []int {
	var pids []int
	seen := make(map[int]bool)
	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err == nil && pid > 0 && !seen[pid] {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}
	return pids
}
