package engine

import (
	"os"
	"path/filepath"
	"testing"
)

type runtimeFixture struct {
	repoRoot string
}

func newRuntimeFixtureRunner(t *testing.T) *runtimeFixture {
	t.Helper()

	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)
	return &runtimeFixture{repoRoot: root}
}

func mustDefaultRuntimeVersion(t *testing.T) string {
	t.Helper()

	version, err := defaultRuntimeVersion()
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
