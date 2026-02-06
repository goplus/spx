package command

import (
	"os"
	"path/filepath"
)

// getShareDir returns the share directory path relative to the executable.
// Expected layout:
//
//	dist/
//	- bin/spx
//	- share/
func getShareDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	binDir := filepath.Dir(exe)
	shareDir := filepath.Join(binDir, "..", "share")
	return filepath.Abs(shareDir)
}

// getLibDir returns the lib directory path relative to the executable.
func getLibDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	binDir := filepath.Dir(exe)
	libDir := filepath.Join(binDir, "..", "lib")
	return filepath.Abs(libDir)
}
