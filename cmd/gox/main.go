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
	cmdtool.RunCmd(cmd, "gdspx", version, proejct_fs, "template/project", "project", "setupweb")
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
	projectDir, _ := filepath.Abs(cmdtool.ProjectDir)
	spxProjPath, _ := filepath.Abs(cmdtool.ProjectDir + "/..")

	rawdir, _ := os.Getwd()
	os.Chdir(spxProjPath)
	envVars := []string{""}
	util.RunGoplus(envVars, "go")
	os.Rename(path.Join(spxProjPath, "gop_autogen.go"), path.Join(cmdtool.GoDir, "main.go"))
	os.Chdir(projectDir)
	util.RunGolang(nil, "mod", "tidy")
	os.Chdir(rawdir)
	cmdtool.BuildDll()
	return nil
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
