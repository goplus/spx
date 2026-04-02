package impl

import (
	"os"
	"os/exec"
	"path"
	"runtime"
)

func CheckAndGetAppPath(gobinDir, tag, version string, customGoEnv bool) (string, string, error) {
	binPostfix := ""
	switch runtime.GOOS {
	case "windows":
		binPostfix = ".exe"
	case "darwin":
		binPostfix = ""
	case "linux":
		binPostfix = ""
	}

	tagName := tag + version
	dstFileName := tagName + binPostfix
	gdx, err := exec.LookPath(dstFileName)
	if err == nil {
		if _, err := exec.Command(gdx, "--version").CombinedOutput(); err == nil {
			return binPostfix, gdx, nil
		}
	}

	dstDir := gobinDir
	cmdPath := path.Join(dstDir, dstFileName)
	info, err := os.Stat(cmdPath)
	if os.IsNotExist(err) {
		if customGoEnv {
			return binPostfix, cmdPath, nil
		}
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
