package spx

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func setupWorkDir() {
	dir := getCurrentAbPathByExecutable()
	if strings.Contains(dir, os.TempDir()) {
		return
	}
	os.Chdir(dir)
}

func getCurrentAbPathByExecutable() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(exePath))
	if err != nil {
		log.Fatal(err)
	}
	return dir
}
