package command

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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

func TestShouldBuildWasmForCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    bool
	}{
		{name: "buildweb", cmdName: "buildweb", want: true},
		{name: "exportweb", cmdName: "exportweb", want: true},
		{name: "runweb", cmdName: "runweb", want: true},
		{name: "runwebworker", cmdName: "runwebworker", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBuildWasmForCommand(tt.cmdName); got != tt.want {
				t.Fatalf("shouldBuildWasmForCommand(%q) = %v, want %v", tt.cmdName, got, tt.want)
			}
		})
	}
}

func TestGetWasmPathsPrefersProjectBuild(t *testing.T) {
	projectDir := t.TempDir()
	cmd := CmdTool{
		ProjectDir: projectDir,
		GoBinPath:  filepath.Join(projectDir, "gobin"),
	}

	projectWasm := filepath.Join(projectDir, ".builds", "web", "ispx.wasm")
	if err := os.MkdirAll(filepath.Dir(projectWasm), 0755); err != nil {
		t.Fatalf("mkdir project wasm dir: %v", err)
	}
	if err := os.WriteFile(projectWasm, []byte("project"), 0644); err != nil {
		t.Fatalf("write project wasm: %v", err)
	}
	if err := os.WriteFile(projectWasm+".br", []byte("project-br"), 0644); err != nil {
		t.Fatalf("write project wasm br: %v", err)
	}

	if err := os.MkdirAll(cmd.GoBinPath, 0755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmd.GoBinPath, "ispx.wasm"), []byte("gobin"), 0644); err != nil {
		t.Fatalf("write gobin wasm: %v", err)
	}

	gotWasm, gotWasmBr := cmd.getWasmPaths()
	if gotWasm != projectWasm {
		t.Fatalf("getWasmPaths wasm = %s, want %s", gotWasm, projectWasm)
	}
	if gotWasmBr != projectWasm+".br" {
		t.Fatalf("getWasmPaths wasm br = %s, want %s", gotWasmBr, projectWasm+".br")
	}
}

func TestGetWasmPathsFallsBackToGoBin(t *testing.T) {
	projectDir := t.TempDir()
	goBinPath := filepath.Join(projectDir, "gobin")
	cmd := CmdTool{
		ProjectDir: projectDir,
		GoBinPath:  goBinPath,
	}

	if err := os.MkdirAll(goBinPath, 0755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	goBinWasm := filepath.Join(goBinPath, "ispx.wasm")
	if err := os.WriteFile(goBinWasm, []byte("gobin"), 0644); err != nil {
		t.Fatalf("write gobin wasm: %v", err)
	}

	gotWasm, gotWasmBr := cmd.getWasmPaths()
	if gotWasm != goBinWasm {
		t.Fatalf("getWasmPaths wasm = %s, want %s", gotWasm, goBinWasm)
	}
	if gotWasmBr != "" {
		t.Fatalf("getWasmPaths wasm br = %s, want empty", gotWasmBr)
	}
}

func TestStopWebIgnoresInvalidPIDFile(t *testing.T) {
	targetDir := t.TempDir()
	cmd := CmdTool{TargetDir: targetDir, TargetAbsDir: targetDir}
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

func TestWebServerPIDPathUsesAbsoluteTargetDir(t *testing.T) {
	targetDir := t.TempDir()
	otherDir := t.TempDir()
	rawDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(rawDir)
	})
	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := CmdTool{TargetDir: ".", TargetAbsDir: targetDir}
	got := cmd.webServerPIDPath()
	want := filepath.Join(targetDir, ".gdspx_web_server.pid")
	if got != want {
		t.Fatalf("webServerPIDPath = %s, want %s", got, want)
	}
}

func TestParsePIDList(t *testing.T) {
	got := parsePIDList([]byte("123\n456\ninvalid\n"))
	want := []int{123, 456}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePIDList = %v, want %v", got, want)
	}
}
