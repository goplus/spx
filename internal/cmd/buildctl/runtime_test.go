package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandRunnerRunCommandUsesGoPathBin(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)

	binDir := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	cmdPath := filepath.Join(binDir, "fakecmd")
	outPath := filepath.Join(root, "ran.txt")
	script := "#!/bin/sh\nprintf 'ok' > \"$OUT_PATH\"\n"
	if err := os.WriteFile(cmdPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("OUT_PATH", outPath)

	runner := commandRunner{repoRoot: root}
	if err := runner.runCommand(".", "fakecmd"); err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	if !fileExists(outPath) {
		t.Fatalf("expected fake command output at %s", outPath)
	}
}

func TestCommandRunnerRunCommandReturnsEnvironmentError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", "")

	runner := commandRunner{repoRoot: root}
	err := runner.runCommand(".", "fakecmd")
	if err == nil {
		t.Fatal("expected runCommand to fail when command environment cannot be resolved")
	}
	if !strings.Contains(err.Error(), "resolve command environment") {
		t.Fatalf("runCommand error = %v, want environment resolution context", err)
	}
}

func TestCommandRunnerRunCommandReturnsResolvePathError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(root, "gopath"))

	runner := commandRunner{repoRoot: root}
	err := runner.runCommand(".", "missingcmd")
	if err == nil {
		t.Fatal("expected runCommand to fail when the command cannot be found")
	}
	if !strings.Contains(err.Error(), "resolve command path for missingcmd") {
		t.Fatalf("runCommand error = %v, want command path resolution context", err)
	}
}
