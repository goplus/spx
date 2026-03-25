package main

import (
	"os"
	"path/filepath"
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
