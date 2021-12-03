package fsutil

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func GetCurrDir() string {
	dir := getCurrentAbPathByExecutable()
	if strings.Contains(dir, os.TempDir()) {
		dir, _ = os.Getwd()
	}
	return dir
}

func getCurrentAbPathByExecutable() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	res, _ := filepath.EvalSymlinks(filepath.Dir(exePath))
	return res
}
