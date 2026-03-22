package command

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRunWebCommandRunsExportBeforeServer(t *testing.T) {
	var calls []string

	err := runWebCommand(
		func() error {
			calls = append(calls, "export")
			return nil
		},
		func() error {
			calls = append(calls, "serve")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runWebCommand returned error: %v", err)
	}

	want := []string{"export", "serve"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected call order: got %v, want %v", calls, want)
	}
}

func TestRunWebCommandStopsOnExportError(t *testing.T) {
	wantErr := errors.New("export failed")
	serverCalled := false

	err := runWebCommand(
		func() error {
			return wantErr
		},
		func() error {
			serverCalled = true
			return nil
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWebCommand error = %v, want %v", err, wantErr)
	}
	if serverCalled {
		t.Fatal("server should not start when export fails")
	}
}

func TestIsRuntimeModeCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    bool
	}{
		{name: "run", cmdName: "run", want: true},
		{name: "runweb", cmdName: "runweb", want: true},
		{name: "runwebworker", cmdName: "runwebworker", want: true},
		{name: "exportweb", cmdName: "exportweb", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRuntimeModeCommand(tt.cmdName); got != tt.want {
				t.Fatalf("isRuntimeModeCommand(%q) = %v, want %v", tt.cmdName, got, tt.want)
			}
		})
	}
}

func TestStopWebIgnoresInvalidPIDFile(t *testing.T) {
	targetDir := t.TempDir()
	cmd := CmdTool{TargetDir: targetDir, ServerPort: freeTCPPort(t)}
	pidFile := filepath.Join(targetDir, ".gdspx_web_server.pid")
	if err := os.WriteFile(pidFile, []byte("invalid"), 0644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	if err := cmd.StopWeb(); err != nil {
		t.Fatalf("StopWeb returned error for invalid pid file: %v", err)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatalf("pid file should be removed, stat err = %v", err)
	}
}

func TestStopWebStopsProcessFromGlobalPIDFile(t *testing.T) {
	port := freeTCPPort(t)
	targetDirA := t.TempDir()
	targetDirB := t.TempDir()
	cmd := CmdTool{TargetDir: targetDirB, ServerPort: port}
	localPIDPath := filepath.Join(targetDirA, ".gdspx_web_server.pid")
	recordBytes := []byte(`{"pid":31337,"localPidPath":"` + localPIDPath + `"}`)
	if err := os.WriteFile(localPIDPath, []byte("31337"), 0644); err != nil {
		t.Fatalf("write local pid file: %v", err)
	}
	if err := os.WriteFile(cmd.globalWebServerPIDPath(), recordBytes, 0644); err != nil {
		t.Fatalf("write global pid file: %v", err)
	}

	origProcessCommandLine := processCommandLineFn
	origKillPID := killPIDFn
	t.Cleanup(func() {
		processCommandLineFn = origProcessCommandLine
		killPIDFn = origKillPID
	})

	processCommandLineFn = func(pid int) (string, error) {
		if pid != 31337 {
			t.Fatalf("unexpected pid lookup: %d", pid)
		}
		return "/usr/bin/python3 /tmp/gdspx_web_server.py -p 8005", nil
	}
	killed := false
	killPIDFn = func(pid int) error {
		if pid != 31337 {
			t.Fatalf("unexpected pid kill: %d", pid)
		}
		killed = true
		return nil
	}

	if err := cmd.StopWeb(); err != nil {
		t.Fatalf("StopWeb returned error: %v", err)
	}
	if !killed {
		t.Fatal("expected StopWeb to kill the process recorded in the global pid file")
	}
	if _, err := os.Stat(localPIDPath); !os.IsNotExist(err) {
		t.Fatalf("local pid file should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(cmd.globalWebServerPIDPath()); !os.IsNotExist(err) {
		t.Fatalf("global pid file should be removed, stat err = %v", err)
	}
}

func TestStopWebKillsGdspxServerByPortWhenPIDFilesMissing(t *testing.T) {
	origIsPortAvailable := isPortAvailableFn
	origFindListeningPIDs := findListeningPIDsFn
	origProcessCommandLine := processCommandLineFn
	origKillPID := killPIDFn
	origWaitForPortFree := waitForPortFreeFn
	t.Cleanup(func() {
		isPortAvailableFn = origIsPortAvailable
		findListeningPIDsFn = origFindListeningPIDs
		processCommandLineFn = origProcessCommandLine
		killPIDFn = origKillPID
		waitForPortFreeFn = origWaitForPortFree
	})

	isPortAvailableFn = func(port int) bool { return false }
	findListeningPIDsFn = func(port int) ([]int, error) { return []int{4242}, nil }
	processCommandLineFn = func(pid int) (string, error) {
		return "/usr/bin/python3 /tmp/gdspx_web_server.py -p 8005", nil
	}
	var killed []int
	killPIDFn = func(pid int) error {
		killed = append(killed, pid)
		return nil
	}
	waitForPortFreeFn = func(port int, timeout time.Duration) error { return nil }

	cmd := CmdTool{TargetDir: t.TempDir(), ServerPort: freeTCPPort(t)}
	if err := cmd.StopWeb(); err != nil {
		t.Fatalf("StopWeb returned error: %v", err)
	}
	if !reflect.DeepEqual(killed, []int{4242}) {
		t.Fatalf("killed pids = %v, want [4242]", killed)
	}
}

func TestStopWebDoesNotKillNonGdspxProcessOnPort(t *testing.T) {
	origIsPortAvailable := isPortAvailableFn
	origFindListeningPIDs := findListeningPIDsFn
	origProcessCommandLine := processCommandLineFn
	origKillPID := killPIDFn
	t.Cleanup(func() {
		isPortAvailableFn = origIsPortAvailable
		findListeningPIDsFn = origFindListeningPIDs
		processCommandLineFn = origProcessCommandLine
		killPIDFn = origKillPID
	})

	isPortAvailableFn = func(port int) bool { return false }
	findListeningPIDsFn = func(port int) ([]int, error) { return []int{5252}, nil }
	processCommandLineFn = func(pid int) (string, error) {
		return "/usr/bin/python3 /tmp/other_server.py -p 8005", nil
	}
	killed := false
	killPIDFn = func(pid int) error {
		killed = true
		return nil
	}

	cmd := CmdTool{TargetDir: t.TempDir(), ServerPort: freeTCPPort(t)}
	if err := cmd.StopWeb(); err == nil {
		t.Fatal("StopWeb should fail when the port is occupied by a non-gdspx process")
	}
	if killed {
		t.Fatal("StopWeb should not kill a non-gdspx process")
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
