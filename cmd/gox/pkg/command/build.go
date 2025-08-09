package command

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

func (pself *CmdTool) BuildWasm() (err error) {
	pself.genGo()
	rawdir, _ := os.Getwd()
	dir := path.Join(pself.ProjectDir, ".builds/web/")
	os.MkdirAll(dir, 0755)
	filePath := path.Join(dir, "gdspx.wasm")
	os.Chdir(pself.GoDir)
	envVars := []string{"GOOS=js", "GOARCH=wasm"}
	util.RunGolang(envVars, "build", "-o", filePath)
	os.Chdir(rawdir)
	return nil
}

// BuildTinyGoLib builds static library using TinyGo for ESP32
func (pself *CmdTool) BuildTinyGoLib() error {
	pself.genGo()
	rawdir, _ := os.Getwd()

	// Create builds directory for tinygo output
	dir := path.Join(pself.ProjectDir, ".builds/tinygo/")
	os.MkdirAll(dir, 0755)

	// Set output file path
	outputPath := path.Join(dir, "golib.o")

	// Change to Go directory
	os.Chdir(pself.GoDir)

	// Determine target board
	target := "esp32"
	if *pself.Args.Target != "" {
		target = *pself.Args.Target
	}
	if target == "esp32" {
		target = "esp32-coreboard-v2"
	}

	// Prepare TinyGo build arguments
	args := []string{
		"build",
		"-o", outputPath,
		"-target=" + target,
		"-no-debug",
		"-opt=2",
		"-gc=leaking",
		"-scheduler=none",
	}

	// Add tags if specified
	if *pself.Args.Tags != "" {
		args = append(args, "-tags="+*pself.Args.Tags)
	}

	// Add current directory as the last argument
	args = append(args, ".") // 使用当前目录，让TinyGo处理所有Go文件

	// Set environment variables including GODEBUG to fix gotypesalias issue
	envVars := []string{"GODEBUG=gotypesalias=0"}

	// Run TinyGo build command with environment variables
	err := util.RunTinyGo(envVars, args...)
	if err != nil {
		log.Printf("TinyGo build failed: %v", err)
		os.Chdir(rawdir)
		return err
	}

	os.Chdir(rawdir)
	log.Printf("TinyGo static library built successfully: %s", outputPath)
	return nil
}

func (pself *CmdTool) BuildDll() error {
	files, _ := filepath.Glob(filepath.Join(pself.ProjectDir, "go", "ios*"))
	// Restore original files
	for _, file := range files {
		if !strings.HasSuffix(file, ".txt") {
			newName := file + ".txt"
			os.Rename(file, newName)
		}
	}

	tarArch := *pself.Args.Arch
	archs := []string{runtime.GOARCH}
	if tarArch != "" {
		if runtime.GOOS == "windows" {
			archs = []string{"amd64", "386"}
		} else if runtime.GOOS == "darwin" {
			archs = []string{"amd64", "arm64"}
		} else if runtime.GOOS == "linux" {
			archs = []string{"amd64", "arm", "arm64", "386"}
		}
		if tarArch != "all" {
			isValid := false
			for _, v := range archs {
				if tarArch == v {
					isValid = true
					break
				}
			}
			if !isValid {
				log.Fatalln("invalid arch "+tarArch, " valid archs:", strings.Join(archs, ","))
			}
			archs = []string{tarArch}
		}
	}
	if runtime.GOOS == "darwin" {
		archs = []string{"amd64", "arm64"}
	}

	rawdir, _ := os.Getwd()
	tagStr := pself.genGo()

	// build dll
	os.Chdir(pself.GoDir)
	envs := []string{"CGO_ENABLED=1"}
	rawPath := filepath.Base(pself.LibPath)
	rawDir := filepath.Dir(pself.LibPath)
	for _, arch := range archs {
		println("build dll arch=", arch, tagStr)
		strs := strings.Split(rawPath, "-")
		posfix := strings.Split(strs[2], ".")
		newPath := rawDir + "/" + strs[0] + "-" + strs[1] + "-" + arch + "." + posfix[len(posfix)-1]
		pself.LibPath = newPath
		envs = append(envs, "GOARCH="+arch)
		if tagStr == "" {
			util.RunGolang(envs, "build", "-o", newPath, "-buildmode=c-shared")
		} else {
			util.RunGolang(envs, "build", tagStr, "-o", newPath, "-buildmode=c-shared")
		}
	}
	os.Chdir(rawdir)
	return nil
}
func (pself *CmdTool) genGo() string {
	rawdir, _ := os.Getwd()
	projectDir, _ := filepath.Abs(pself.ProjectDir)
	spxProjPath, _ := filepath.Abs(pself.ProjectDir + "/..")

	os.Chdir(spxProjPath)
	envVars := []string{""}
	tagStr := ""
	if *pself.Args.Tags != "" {
		tagStr = "-tags=" + *pself.Args.Tags
	}
	log.Printf("genGo tagStr: %s", tagStr)

	if tagStr == "" {
		util.RunXGo(envVars, "go")
	} else {
		util.RunXGo(envVars, "go", tagStr)
	}

	os.MkdirAll(pself.GoDir, 0755)
	os.Rename(path.Join(spxProjPath, "xgo_autogen.go"), path.Join(pself.GoDir, "main.go"))
	os.Chdir(projectDir)
	util.RunGolang(nil, "mod", "tidy")
	os.Chdir(rawdir)
	return tagStr
}
