package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/scaffold"
)

func TestGenerateRuntimeTemplatesFromNestedDirectory(t *testing.T) {
	dir := newTemplateRepo(t)
	t.Chdir(dir)
	for range 2 {
		main()
		for name, want := range map[string]string{
			"runtime.gdextension.txt": scaffold.RuntimeGDExtension(),
			"gdspx.gdextension":       scaffold.ProjectGDExtension(),
		} {
			got, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Fatalf("%s does not match the scaffold template", name)
			}
		}
	}
}

func TestGenerateRuntimeTemplatesReportsMissingRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := run(); err == nil || !strings.HasPrefix(err.Error(), "find repo root: go.mod not found from ") {
		t.Fatalf("run() = %v, want missing repository root error", err)
	}
}

func TestGenerateRuntimeTemplatesReportsWriteFailure(t *testing.T) {
	dir := newTemplateRepo(t)
	if err := os.Mkdir(filepath.Join(dir, "runtime.gdextension.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "write " + filepath.Join("cmd", "spx", "template", "project", "runtime.gdextension.txt") + ": "
	if err := run(); err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("run() = %v, want failed output path %q", err, want)
	}
}

func TestGenerateRuntimeTemplatesReportsRemovedWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Remove(dir); err != nil {
		t.Skipf("cannot remove current directory on this platform: %v", err)
	}
	if _, err := os.Getwd(); err == nil {
		t.Skip("this platform can still resolve a removed current directory")
	}
	if err := run(); !errors.Is(err, os.ErrNotExist) || !strings.HasPrefix(err.Error(), "find repo root: ") {
		t.Fatalf("run() = %v, want working directory error", err)
	}
}

func newTemplateRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile("go.mod", []byte("module example.com/runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "cmd", "spx", "template", "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGenerateRuntimeTemplatesMainReportsFailure(t *testing.T) {
	const helper = "SPX_TEST_TEMPLATE_MAIN_ERROR"
	if os.Getenv(helper) == "1" {
		main()
		return
	}
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestGenerateRuntimeTemplatesMainReportsFailure$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), helper+"=1")
	output, err := cmd.Output()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || len(output) != 0 {
		t.Fatalf("main = %v, stdout = %q; want exit 1 without output", err, output)
	}
	if !strings.Contains(string(exit.Stderr), "find repo root: go.mod not found") {
		t.Fatalf("stderr = %q; want missing repository diagnostic", exit.Stderr)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("failed generation left files: %v, %v", entries, err)
	}
}
