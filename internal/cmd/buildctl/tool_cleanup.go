package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func cleanInstalledAssets() error {
	goPath, err := ensureGoPath()
	if err != nil {
		return err
	}

	binDir := filepath.Join(goPath, "bin")
	if !fileExists(binDir) {
		fmt.Fprintf(os.Stdout, "No GOPATH bin directory found at %s\n", binDir)
		return nil
	}

	patterns := []string{
		"spx",
		"spx.exe",
		"ispx",
		"ispx.wasm",
		"ispx.wasm.br",
		"runtime.gdextension",
		"gdspx*",
	}

	seen := map[string]struct{}{}
	var removed []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(binDir, pattern))
		if err != nil {
			return err
		}
		for _, match := range matches {
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			if err := os.RemoveAll(match); err != nil {
				return fmt.Errorf("remove installed asset %s: %w", match, err)
			}
			removed = append(removed, match)
		}
	}

	if len(removed) == 0 {
		fmt.Fprintf(os.Stdout, "No installed SPX assets found under %s\n", binDir)
		return nil
	}

	sort.Strings(removed)
	for _, path := range removed {
		fmt.Fprintf(os.Stdout, "Removed %s\n", path)
	}
	return nil
}
