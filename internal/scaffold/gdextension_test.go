package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGDExtensionTemplatesMatchProjectFiles(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	assertTemplateFile(t, filepath.Join(repoRoot, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), RuntimeGDExtension())
	assertTemplateFile(t, filepath.Join(repoRoot, "cmd", "spx", "template", "project", "gdspx.gdextension"), ProjectGDExtension())
}

func assertTemplateFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of sync; run go generate ./cmd/spx", path)
	}
}
