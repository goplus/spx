package impl

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "embed"
)

func CheckAndGetAppPath(engineDir, tag, version string) (string, string, error) {
	binPostfix := ""
	if runtime.GOOS == "windows" {
		binPostfix = ".exe"
	} else if runtime.GOOS == "darwin" {
		binPostfix = ""
	} else if runtime.GOOS == "linux" {
		binPostfix = ""
	}

	tagName := tag + version
	dstFileName := tagName + binPostfix
	cmdPath := filepath.Join(engineDir, dstFileName)
	info, err := os.Stat(cmdPath)
	if os.IsNotExist(err) {
		return binPostfix, cmdPath, fmt.Errorf("engine binary not found: %s", cmdPath)
	} else if err != nil {
		return binPostfix, "", err
	} else {
		if info.Mode()&0111 == 0 {
			if err := os.Chmod(cmdPath, 0755); err != nil {
				return binPostfix, cmdPath, err
			}
		}
	}
	return binPostfix, cmdPath, nil
}
