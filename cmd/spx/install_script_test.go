package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScriptUsesExistingGenerateTarget(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	dir := filepath.Dir(file)

	scriptPath := filepath.Join(dir, "install.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}

	const generateTarget = "internal/gengo/embedded_pkgs.go"
	if !strings.Contains(string(content), "go generate "+generateTarget) {
		t.Fatalf("install script should generate %s", generateTarget)
	}

	if _, err := os.Stat(filepath.Join(dir, generateTarget)); err != nil {
		t.Fatalf("generate target %s missing: %v", generateTarget, err)
	}
}
