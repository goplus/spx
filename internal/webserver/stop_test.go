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

package webserver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestCommandFixture supplies portable process output/exit codes without signals.
func TestCommandFixture(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "--webserver-fixture" {
		return
	}
	fmt.Print("fixture output")
	if os.Args[len(os.Args)-1] == "fail" {
		os.Exit(1)
	}
	os.Exit(0)
}

func fixtureExitError(t *testing.T) error {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stopper := New("linux")
	output, err := stopper.output(executable, "-test.run=^TestCommandFixture$", "--", "--webserver-fixture", "ok")
	if err != nil || string(output) != "fixture output" {
		t.Fatalf("default output = %q, %v", output, err)
	}
	_, err = stopper.output(executable, "-test.run=^TestCommandFixture$", "--", "--webserver-fixture", "fail")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("fixture error = %v", err)
	}
	return err
}

func TestStop(t *testing.T) {
	denied := errors.New("permission denied")
	for _, goos := range []string{"linux", "windows"} {
		for _, tt := range []struct {
			name, line        string
			pid               int
			queryErr, killErr error
			wantStop          bool
		}{
			{name: "zero pid", pid: 0},
			{name: "negative pid", pid: -1},
			{name: "missing process", pid: 37},
			{name: "unrelated python", pid: 37, line: "python unrelated.py"},
			{name: "script prefix collision", pid: 37, line: "python not_gdspx_web_server.py"},
			{name: "script suffix collision", pid: 37, line: "python gdspx_web_server.py.bak"},
			{name: "quoted path with spaces", pid: 37, line: `"C:\Program Files\Python\python.exe" "C:\SPX Projects\gdspx_web_server.py" -p 8060`, wantStop: true},
			{name: "single quoted Unix path", pid: 37, line: `python '/tmp/SPX Projects/gdspx_web_server.py' -p 8060`, wantStop: true},
			{name: "failed query discards output", pid: 37, line: "python gdspx_web_server.py", queryErr: denied},
			{name: "matching server", pid: 37, line: "python gdspx_web_server.py -p 8060", wantStop: true},
			{name: "case sensitive on Unix", pid: 37, line: `python.exe C:\tmp\GDSPX_WEB_SERVER.PY`, wantStop: goos == "windows"},
			{name: "kill denied", pid: 37, line: "python gdspx_web_server.py", killErr: denied},
		} {
			t.Run(goos+"/"+tt.name, func(t *testing.T) {
				var calls []string
				stopper := New(goos)
				stubTermination(&stopper)
				stopper.output = func(name string, args ...string) ([]byte, error) {
					calls = append(calls, name+" "+strings.Join(args, " "))
					if len(calls) == 1 {
						want := "ps -o args= -p 37"
						if goos == "windows" {
							want = `powershell -NoProfile -NonInteractive -Command (Get-CimInstance Win32_Process -Filter "ProcessId = 37").CommandLine`
						}
						if calls[0] != want {
							t.Fatalf("query = %q, want %q", calls[0], want)
						}
						return []byte(tt.line), tt.queryErr
					}
					want := "kill -KILL 37"
					if goos == "windows" {
						want = "taskkill /F /PID 37"
					}
					if len(calls) != 2 || calls[1] != want {
						t.Fatalf("kill = %v, want %q", calls, want)
					}
					return nil, tt.killErr
				}
				stopped, err := stopper.Stop(tt.pid)
				if stopped != tt.wantStop || !errors.Is(err, tt.killErr) {
					t.Fatalf("Stop = %v, %v; want %v, %v", stopped, err, tt.wantStop, tt.killErr)
				}
				wantCalls := 1
				if tt.pid <= 0 {
					wantCalls = 0
				} else if tt.wantStop || tt.killErr != nil {
					wantCalls = 2
				}
				if len(calls) != wantCalls {
					t.Fatalf("calls = %v, want %d", calls, wantCalls)
				}
			})
		}
	}
}

func TestStopAll(t *testing.T) {
	exitOne := fixtureExitError(t)
	denied := errors.New("permission denied")
	for _, goos := range []string{"linux", "windows"} {
		for _, tt := range []struct {
			name, output      string
			queryErr, killErr error
			wantErr           bool
			wantKilled        []string
		}{
			{name: "empty"},
			{name: "invalid candidates", output: "0 -1 invalid 999999999999999999999999"},
			{name: "no match exit", queryErr: exitOne, wantErr: goos == "windows"},
			{name: "discovery failure", output: "partial 31", queryErr: denied, wantErr: true},
			{name: "verify each candidate", output: "31 42 31 43 44", wantKilled: []string{"31", "44"}},
			{name: "continue after kill failure", output: "31 42 31 43 44", killErr: denied, wantErr: true, wantKilled: []string{"31", "44"}},
		} {
			t.Run(goos+"/"+tt.name, func(t *testing.T) {
				stopper := New(goos)
				stubTermination(&stopper)
				var calls, killed []string
				stopper.output = func(name string, args ...string) ([]byte, error) {
					call := name + " " + strings.Join(args, " ")
					calls = append(calls, call)
					if len(calls) == 1 {
						want := "pgrep -f gdspx_web_server.py"
						if goos == "windows" {
							want = `powershell -NoProfile -NonInteractive -Command Get-CimInstance Win32_Process -Filter "CommandLine LIKE '%gdspx_web_server.py%'" | Select-Object -ExpandProperty ProcessId`
						}
						if call != want {
							t.Fatalf("discovery = %q, want %q", call, want)
						}
						return []byte(tt.output), tt.queryErr
					}
					if name == "kill" || name == "taskkill" {
						pid := args[len(args)-1]
						killed = append(killed, pid)
						if pid == "31" {
							return nil, tt.killErr
						}
						return nil, nil
					}
					if strings.Contains(call, "43") {
						return []byte("python gdspx_web_server.py"), denied
					}
					if strings.Contains(call, "42") {
						return []byte("python unrelated.py"), nil
					}
					return []byte("python gdspx_web_server.py"), nil
				}
				err := stopper.StopAll()
				if (err != nil) != tt.wantErr {
					t.Fatalf("StopAll = %v, want error %v", err, tt.wantErr)
				}
				if !reflect.DeepEqual(killed, tt.wantKilled) {
					t.Fatalf("killed = %v, want %v", killed, tt.wantKilled)
				}
				if tt.killErr != nil && !errors.Is(err, tt.killErr) {
					t.Fatalf("kill error not preserved: %v", err)
				}
				if tt.wantKilled == nil && len(calls) != 1 {
					t.Fatalf("unexpected process action: %v", calls)
				}
			})
		}
	}
}

func TestListeningPIDs(t *testing.T) {
	exitOne := fixtureExitError(t)
	failed := errors.New("query failed")
	for _, goos := range []string{"linux", "windows"} {
		for _, tt := range []struct {
			name    string
			port    int
			err     error
			wantErr bool
		}{
			{name: "zero port"}, {name: "negative port", port: -1},
			{name: "parsed", port: 8060},
			{name: "no listeners exit", port: 8060, err: exitOne, wantErr: goos == "windows"},
			{name: "query error", port: 8060, err: failed, wantErr: true},
		} {
			t.Run(goos+"/"+tt.name, func(t *testing.T) {
				called := false
				stopper := New(goos)
				stubTermination(&stopper)
				stopper.output = func(name string, args ...string) ([]byte, error) {
					called = true
					want := "lsof -nP -t -iTCP:8060 -sTCP:LISTEN"
					output := "31 invalid 42 31 0 -1 999999999999999999999"
					if goos == "windows" {
						want = "netstat -ano -p tcp"
						output = "Proto Local Address Foreign Address State PID\n" +
							"UDP 0.0.0.0:8060 *:* LISTENING 10\nTCP 0.0.0.0:8061 *:* LISTENING 11\n" +
							"TCP 0.0.0.0:8060 *:* ESTABLISHED 12\nTCP 0.0.0.0:8060 *:* LISTENING invalid\n" +
							"TCP 0.0.0.0:8060 *:* LISTENING 31\ntcp [::]:8060 *:* listen 42\n" +
							"TCP 0.0.0.0:8060 *:* LISTENING 31\nTCP 0.0.0.0:8060 *:* LISTENING 0\nincomplete\n"
					}
					if got := name + " " + strings.Join(args, " "); got != want {
						t.Fatalf("query = %q, want %q", got, want)
					}
					return []byte(output), tt.err
				}
				pids, err := stopper.ListeningPIDs(tt.port)
				var want []int
				if tt.port > 0 && tt.err == nil {
					want = []int{31, 42}
				}
				if (err != nil) != tt.wantErr || !reflect.DeepEqual(pids, want) || called != (tt.port > 0) {
					t.Fatalf("ListeningPIDs = %v, %v, called %v; want %v, error %v", pids, err, called, want, tt.wantErr)
				}
			})
		}
	}
}

// All kill paths are intercepted; no tests signal a real process.
func stubTermination(stopper *Stopper) {
	stopper.terminate = func(pid int) error {
		_, err := stopper.output("kill", "-KILL", strconv.Itoa(pid))
		return err
	}
}
