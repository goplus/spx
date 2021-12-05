package fsutil

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func GetWorkDir() string {
	dir := getCurrentAbPathByExecutable()
	if strings.Contains(dir, os.TempDir()) {
		dir, _ = os.Getwd()
	}
	return dir
}

// 获取当前执行程序所在的绝对路径
func getCurrentAbPathByExecutable() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	res, _ := filepath.EvalSymlinks(filepath.Dir(exePath))
	return res
}
