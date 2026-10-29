package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v4/internal/scaffold"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("find repo root: %w", err)
	}

	files := map[string]string{
		filepath.Join("cmd", "spx", "template", "project", "runtime.gdextension.txt"): scaffold.RuntimeGDExtension(),
		filepath.Join("cmd", "spx", "template", "project", "gdspx.gdextension"):       scaffold.ProjectGDExtension(),
	}

	for path, content := range files {
		fullPath := filepath.Join(repoRoot, path)
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
