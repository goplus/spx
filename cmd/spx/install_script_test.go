package main

import (
	"os"
	"strings"
	"testing"
)

func TestInstallScriptUsesExistingGenerateTarget(t *testing.T) {
	content, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}

	const generateTarget = "internal/gengo/embedded_pkgs.go"
	if !strings.Contains(string(content), "go generate "+generateTarget) {
		t.Fatalf("install script should generate %s", generateTarget)
	}

	if _, err := os.Stat(generateTarget); err != nil {
		t.Fatalf("generate target %s missing: %v", generateTarget, err)
	}
}
