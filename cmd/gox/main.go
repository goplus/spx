package main

import (
	"archive/zip"
	"crypto/sha256"
	"embed"
	_ "embed"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/realdream-ai/gdspx/cmd/gdspx/pkg/cmdtool"
	"github.com/realdream-ai/gdspx/cmd/gdspx/pkg/util"
)

var (
	//go:embed template/project/*
	proejct_fs embed.FS

	//go:embed template/version
	version string

	//go:embed template/.gitignore.txt
	gitignore_txt string

	rawProjPath string
	webDir      string
)

type CmdTool struct {
	cmdtool.BaseCmdTool
}

const PROJECT_NAME = "spx"
const PROJECT_FILE_SUFFIX = "spx"

func main() {
	cmd := &CmdTool{}
	if os.Args[1] == "setupweb" {
		setupweb()
		return
	}
	cmdtool.RunCmd(cmd, "gdspx", version, proejct_fs, "template/project", "project", "setupweb", "exportapk")
	if os.Args[1] == "exportapk" {
		cmd.exportApk()
		return
	}
}

func setupweb() {
	filePath := getWasmPath()
	rawdir, _ := os.Getwd()
	os.Chdir("../igox")
	envVars := []string{"GOOS=js", "GOARCH=wasm"}
	util.RunGolang(envVars, "build", "-o", filePath)
	os.Chdir(rawdir)
}

func (pself *CmdTool) CheckEnv() error {
	dir, _ := filepath.Abs(cmdtool.TargetDir)

	exist := CheckFileExist(dir, PROJECT_FILE_SUFFIX, false)
	if !exist {
		return fmt.Errorf("can not find " + PROJECT_FILE_SUFFIX + " file, not a valid project dir")
	}
	return nil
}
func CheckFileExist(dir, ext string, recursive bool) bool {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	if recursive {
		// Recursive search using filepath.Walk
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ext) {
				return fmt.Errorf("file found")
			}
			return nil
		})

		if err != nil && err.Error() == "file found" {
			return true
		}
	} else {
		// Non-recursive search, only check the top-level directory
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
				return true
			}
		}
	}

	return false
}
func (pself *CmdTool) OnBeforeCheck(cmd string) error {
	webDir, _ = filepath.Abs(filepath.Join(cmdtool.ProjectDir, ".builds", "web"))
	return nil
}

func (pself *CmdTool) Clear() {
	os.RemoveAll(cmdtool.ProjectDir)
	os.Remove(path.Join(cmdtool.ProjectDir, "../.gitignore"))
}

func (pself *CmdTool) ExportWeb() error {
	pself.Clear()
	installProject()

	err := cmdtool.ExportWebEditor()
	util.CopyDir(proejct_fs, "template/project/.builds/web", webDir, true)
	packProject(cmdtool.TargetDir, path.Join(webDir, "game.zip"))
	packEngineRes(webDir)
	util.CopyFile(getWasmPath(), path.Join(webDir, "gdspx.wasm"))
	saveEngineHash(webDir)
	return err
}
func installProject() {
	// copy project files
	util.CopyDir(proejct_fs, "template/project", cmdtool.ProjectDir, true)
	dir := cmdtool.TargetDir
	util.SetupFile(false, path.Join(dir, ".gitignore"), gitignore_txt)
	os.Rename(path.Join(dir, ".gitignore.txt"), path.Join(dir, ".gitignore"))
}

func (pself *CmdTool) RunWeb() error {
	if !util.IsFileExist(filepath.Join(cmdtool.ProjectDir, ".builds", "web", "engineres.zip")) {
		pself.ExportWeb()
	}
	return cmdtool.RunWeb()
}

func (pself *CmdTool) BuildDll() error {
	pself.parseGop()
	cmdtool.BuildDll()
	return nil
}

func (*CmdTool) parseGop() {
	projectDir, _ := filepath.Abs(cmdtool.ProjectDir)
	spxProjPath, _ := filepath.Abs(cmdtool.ProjectDir + "/..")

	os.MkdirAll(cmdtool.GoDir, 0755)
	rawdir, _ := os.Getwd()
	os.Chdir(spxProjPath)
	println(spxProjPath)
	envVars := []string{""}
	util.RunGoplus(envVars, "go")
	os.Rename(path.Join(spxProjPath, "gop_autogen.go"), path.Join(cmdtool.GoDir, "main.go"))
	os.Chdir(projectDir)
	util.RunGolang(nil, "mod", "tidy")
	os.Chdir(rawdir)
}

func getWasmPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	targetPath := path.Join(gopath, "bin")
	filePath := path.Join(targetPath, "i"+PROJECT_NAME+".wasm")
	return filePath
}

type DirInfos struct {
	path string
	info os.FileInfo
}

func packProject(baseFolder string, dstZipPath string) {
	paths := []DirInfos{}
	if util.IsFileExist(dstZipPath) {
		os.Remove(dstZipPath)
	}
	skipDirs := map[string]struct{}{
		".git": {}, "project": {},
	}

	file, err := os.Create(dstZipPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	err = filepath.Walk(baseFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the path is directly under the base folder
		rel, err := filepath.Rel(baseFolder, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// skip .import files
		if strings.HasSuffix(path, ".import") {
			return nil
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) == 1 || (len(parts) == 2 && info.IsDir()) {
			// Check if the file or directory is in the skip list
			if _, ok := skipDirs[info.Name()]; ok {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		paths = append(paths, DirInfos{path, info})
		return nil
	})
	if err != nil {
		panic(err)
	}

	packZip(zipWriter, baseFolder, paths)
}

func packZip(zipWriter *zip.Writer, baseFolder string, paths []DirInfos) {
	baseFolder = strings.ReplaceAll(baseFolder, "\\", "/")
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].path < paths[j].path
	})
	for _, dirInfo := range paths {
		path := dirInfo.path
		path = strings.ReplaceAll(path, "\\", "/")
		info := dirInfo.info
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			panic(err)
		}
		// Set a fixed timestamp
		header.Modified = time.Unix(0, 0)

		header.Name = strings.TrimPrefix(path, baseFolder)
		header.Name = strings.ReplaceAll(header.Name, "\\", "/")
		if header.Name[0] == '/' {
			header.Name = header.Name[1:]
		}
		if info.IsDir() {
			header.Name += "/"
			_, err := zipWriter.CreateHeader(header)
			if err != nil {
				panic(err)
			}
			continue
		}

		fileToZip, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		defer fileToZip.Close()

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(writer, fileToZip)
		if err != nil {
			panic(err)
		}
	}
}

func computeHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	hashBytes := hasher.Sum(nil)
	return fmt.Sprintf("%x", hashBytes), nil
}
func saveEngineHash(webDir string) {
	// calc and save wasm hash
	files := []string{"gdspx.wasm", "godot.editor.wasm"}
	outpuString := `
function GetEngineHashes() { 
	return {
#HASHES
	}
}
	`
	line := ""
	for _, file := range files {
		hash, err := computeHash(path.Join(webDir, file))
		if err != nil {
			fmt.Printf("Error computing hash for %s: %v\n", file, err)
			continue
		}
		line += fmt.Sprintf("\"%s\":\"%s\",\n", file, hash)
	}
	js := strings.Replace(outpuString, "#HASHES", line, -1)

	// append to game.js
	file, err := os.OpenFile(path.Join(webDir, "game.js"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if _, err := file.WriteString(js); err != nil {
		panic(err)
	}
}

func packEngineRes(webDir string) {
	dstDir := path.Join(webDir, "project")
	util.CopyDir(proejct_fs, "template/project", dstDir, true)

	directories := []string{"engine"}
	files := []string{"main.tscn", "project.godot"}
	err := packDirFiles(path.Join(webDir, "engineres.zip"), dstDir, directories, files)
	if err != nil {
		panic(err)
	}
	os.RemoveAll(dstDir)
}

func packDirFiles(zipName string, targetDir string, directories, files []string) error {
	zipFile, err := os.Create(zipName)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()
	paths := []DirInfos{}
	for _, dir := range directories {
		paths = addDirToZip(path.Join(targetDir, dir), paths)
	}

	for _, file := range files {
		paths = addFileToZip(path.Join(targetDir, file), paths)
	}

	packZip(zipWriter, targetDir, paths)
	return nil
}

func addDirToZip(dirPath string, paths []DirInfos) []DirInfos {
	filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, DirInfos{path, info})
		return nil
	})
	return paths
}

func addFileToZip(path string, paths []DirInfos) []DirInfos {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	paths = append(paths, DirInfos{path, info})
	return paths
}

func (pself *CmdTool) exportApk() {
	pself.BuildDll()
	// parse gop
	pself.parseGop()
	// copy assets
	projectDir, _ := filepath.Abs(cmdtool.ProjectDir)
	copyDir(path.Join(projectDir, "../assets"), path.Join(projectDir, "assets"))

	apkPath := path.Join(projectDir, ".builds/game.apk")
	// run build script
	err := pself.buildAndInstallAPK(projectDir, apkPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	// Check if APK was successfully generated
	if _, err := pself.installApk(apkPath); err != nil {
		fmt.Println(err)
		return
	}
}

func copyDir(src string, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create the destination directory
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Copy file
			err = util.CopyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// BuildAndInstallAPK builds and installs the Godot project on an Android device.
func (pself *CmdTool) buildAndInstallAPK(defaultProjDir, apkPath string) error {
	// Determine the project directory
	absProjDir := defaultProjDir
	println(absProjDir)
	// Check if ANDROID_NDK_ROOT is set
	androidNDKRoot := os.Getenv("ANDROID_NDK_ROOT")
	if androidNDKRoot == "" {
		return fmt.Errorf("error: ANDROID_NDK_ROOT environment variable is not set")
	}

	// Detect system architecture and OS
	hostTag, err := detectHostTag()
	if err != nil {
		return err
	}

	ndkToolchain := filepath.Join(androidNDKRoot, "toolchains/llvm/prebuilt", hostTag, "bin")
	minSDK := "21"

	// Change to the Go directory
	goDir := filepath.Join(absProjDir, "go")
	libDir := filepath.Join(absProjDir, "lib")

	if err := os.Chdir(goDir); err != nil {
		return fmt.Errorf("failed to change to Go directory: %s %v", goDir, err)
	}

	// Build for arm64-v8a
	funcBuildGo := func(dstFile string, arch, ccName string) error {
		envVars := []string{"CGO_ENABLED=1", "GOOS=android", "GOARCH=" + arch, "CC=" + filepath.Join(ndkToolchain, ccName)}
		return util.RunGolang(envVars, "build", "-tags=packmode", "-buildmode=c-shared",
			"-o", filepath.Join(libDir, dstFile),
			"main.go",
		)
	}

	fmt.Println("Building for arm64-v8a...")
	if err := funcBuildGo("libgdspx-android-arm64.so", "arm64", "aarch64-linux-android"+minSDK+"-clang"); err != nil {
		return err
	}

	// Build for armeabi-v7a
	fmt.Println("Building for armeabi-v7a...")
	if err := funcBuildGo("libgdspx-android-arm32.so", "arm", "armv7a-linux-androideabi"+minSDK+"-clang"); err != nil {
		return err
	}

	fmt.Println("Build android so completed successfully!")

	// Check if GODOT_BIN is set
	cmdBin := cmdtool.CmdPath
	if cmdBin == "" {
		return fmt.Errorf("error: GODOT_BIN environment variable is not set")
	}

	// Determine the Godot project path
	projectPath := filepath.Join(absProjDir, "project.godot")
	buildDir := filepath.Dir(apkPath)

	// Create builds directory if it does not exist
	if err := os.MkdirAll(buildDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create builds directory: %v", err)
	}

	// Check if Godot binary exists
	if _, err := os.Stat(cmdBin); os.IsNotExist(err) {
		return fmt.Errorf("error: Godot binary not found: %s", cmdBin)
	}

	// Check if Godot project file exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("error: Godot project file not found: %s", projectPath)
	}

	// Import Godot project resources
	fmt.Println("Importing project resources...")
	if err := runCommand(cmdBin, "--headless", "--path", absProjDir, "--editor", "--quit"); err != nil {
		return err
	}

	// Export the Godot project to APK
	fmt.Println("Exporting Godot project to APK...")
	if err := runCommand(cmdBin, "--headless", "--path", absProjDir, "--export-debug", "Android", apkPath); err != nil {
		return err
	}

	return nil
}

func (pself *CmdTool) installApk(apkPath string) (bool, error) {
	if _, err := os.Stat(apkPath); os.IsNotExist(err) {
		return true, fmt.Errorf("error: APK export failed")
	}

	// Check if adb is available
	if _, err := exec.LookPath("adb"); err != nil {
		return true, fmt.Errorf("error: adb command not found. Please ensure Android SDK platform tools are installed and in PATH")
	}

	// Check if an Android device is connected
	adbDevicesOutput, err := exec.Command("adb", "devices").Output()
	if err != nil || !strings.Contains(string(adbDevicesOutput), "device") {
		return true, fmt.Errorf("error: No Android device connected. Please connect a device and enable USB debugging")
	}

	// Install the APK
	fmt.Println("Installing APK...")
	if err := runCommand("adb", "install", "-r", apkPath); err != nil {
		return true, err
	}

	fmt.Println("APK installation successful!")
	return false, nil
}

// runCommand executes a command and prints its output
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// detectHostTag detects the current system's NDK prebuilt directory
func detectHostTag() (string, error) {
	osName, err := exec.Command("uname", "-s").Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect operating system: %v", err)
	}
	arch, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect architecture: %v", err)
	}

	osStr := strings.TrimSpace(string(osName))
	archStr := strings.TrimSpace(string(arch))

	switch osStr {
	case "Linux":
		if archStr == "x86_64" {
			return "linux-x86_64", nil
		} else if archStr == "aarch64" {
			return "linux-aarch64", nil
		}
	case "Darwin":
		if archStr == "x86_64" {
			return "darwin-x86_64", nil
		} else if archStr == "arm64" {
			return "darwin-aarch64", nil
		}
	default:
		if strings.Contains(osStr, "MINGW") || strings.Contains(osStr, "MSYS") || strings.Contains(osStr, "CYGWIN") {
			if archStr == "x86_64" || archStr == "amd64" {
				return "windows-x86_64", nil
			}
		}
	}

	return "", fmt.Errorf("unsupported operating system or architecture: %s - %s", osStr, archStr)
}
