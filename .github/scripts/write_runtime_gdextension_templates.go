package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v2/internal/scaffold"
)

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)
		os.Exit(1)
	}

	files := map[string]string{
		filepath.Join("cmd", "spx", "template", "project", "runtime.gdextension.txt"): scaffold.RuntimeGDExtension(),
		filepath.Join("cmd", "spx", "template", "project", "gdspx.gdextension"):       scaffold.ProjectGDExtension(),
	}

	for path, content := range files {
		fullPath := filepath.Join(repoRoot, path)
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
	}
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
