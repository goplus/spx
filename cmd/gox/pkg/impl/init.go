package impl

import (
	"os"
	"os/exec"
	"path"
	"runtime"

	_ "embed"

	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

func downloadPack(dstDir, tagName, postfix string) error {
	urlHeader := "https://github.com/JiepengTan/godot/releases/download/"
	fileName := tagName + postfix
	url := urlHeader + tagName + "/" + fileName
	// download pc
	err := util.DownloadFile(url, path.Join(dstDir, fileName))
	if err != nil {
		return err
	}
	// download web
	fileName = tagName + "_web.zip"
	url = urlHeader + tagName + "/" + fileName
	err = util.DownloadFile(url, path.Join(dstDir, fileName))
	if err != nil {
		return err
	}
	// download webpack
	fileName = tagName + "_webpack.zip"
	url = urlHeader + tagName + "/" + fileName
	err = util.DownloadFile(url, path.Join(dstDir, fileName))
	if err != nil {
		return err
	}
	return err
}

func CheckAndGetAppPath(gobinDir, tag, version string) (string, string, error) {
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
		println("Engine is not exist , please download or build engine from source ...", cmdPath)
		os.Exit(1)
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
