package shared

import (
	"fmt"
	"os"
	"path/filepath"
)

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if isRepoRoot(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repository root from %s", dir)
		}
		dir = parent
	}
}

func isRepoRoot(dir string) bool {
	return fileExists(filepath.Join(dir, "go.mod")) &&
		fileExists(filepath.Join(dir, "cmd", "spx", "install.sh")) &&
		fileExists(filepath.Join(dir, "internal", "tools"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
