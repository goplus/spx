package main

import (
	"archive/zip"
	"crypto/sha256"
	"embed"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/realdream-ai/gdspx/cmd/gdspx/pkg/impl"

	_ "embed"
)

var (
	//go:embed template/project/*
	engineFiles embed.FS

	//go:embed template/.gitignore.txt
	gitignore_txt string

	rawProjPath string
)

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
func main() {
	impl.CheckPresetEnvironment()
	impl.TargetDir = "."
	if len(os.Args) > 2 {
		impl.TargetDir = os.Args[2]
	}
	rawProjPath, _ = filepath.Abs(impl.TargetDir)
	impl.TargetDir, _ = filepath.Abs(path.Join(impl.TargetDir, "project"))

	if len(os.Args) <= 1 {
		showHelpInfo()
		return
	}
	if len(os.Args) > 3 {
		port := os.Args[3]
		impl.ServerPort, _ = strconv.Atoi(port)
	}
	if !stringInSlice(os.Args[1], []string{"help", "version", "init", "run", "editor", "build",
		"export", "runweb", "buildweb", "exportweb", "clear",
		"exporti", "runi", "clearbuild", "stopweb", "installispx"}) {
		println("invalid cmd, please refer to help")
		showHelpInfo()
		return
	}

	switch os.Args[1] {
	case "help", "version":
		showHelpInfo()
		return
	case "clearbuild":
		impl.StopWebServer()
		os.RemoveAll(path.Join(impl.TargetDir, ".builds"))
		return
	case "installispx":
		installISpx()
		return
	case "clear":
		impl.StopWebServer()
		clearProject()
		return
	case "stopweb":
		impl.StopWebServer()
		return
	}

	if err := execCmds(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func execCmds() error {
	initProject()
	gdspxPath, project, libPath, err := impl.SetupEnv()
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "init":
		return nil
	case "run", "editor", "export", "build":
		buildDll(project, libPath)
	}
	webDir, _ := filepath.Abs(path.Join(impl.TargetDir, ".builds/web"))
	switch os.Args[1] {
	case "editor":
		return impl.RunGdspx(gdspxPath, project, "-e")
	case "export":
		return impl.Export(gdspxPath, project)
	case "run":
		return impl.RunGdspx(gdspxPath, project, "")
	case "exportweb":
		return exportWeb(webDir)
	case "runweb":
		return runWeb(webDir)
	}
	return nil
}

func initProject() {
	if impl.IsFileExist(impl.TargetDir) {
		return
	}
	dir := impl.TargetDir
	if !impl.IsFileExist(dir) {
		impl.CopyEmbed(engineFiles, "template/project", dir)
	}
	impl.SetupFile(false, path.Join(dir, "../.gitignore"), gitignore_txt)
	os.Rename(path.Join(dir, "../.gitignore.txt"), path.Join(dir, "../.gitignore"))
	os.Rename(path.Join(dir, ".gitignore.txt"), path.Join(dir, ".gitignore"))
	os.Rename(path.Join(dir, "go.mod.txt"), path.Join(dir, "go.mod"))
}

func exportWeb(webDir string) error {
	clearProject()
	initProject()
	err := impl.ExportWebEditor(impl.GdspxPath, impl.ProjectPath, impl.LibPath)
	impl.CopyEmbed(engineFiles, "template/project/.builds/web", webDir)
	packProject(rawProjPath, path.Join(webDir, "game.zip"))
	packEngineRes(webDir)
	impl.CopyFile(getISpxPath(), path.Join(webDir, "gdspx.wasm"))
	saveEngineHash(webDir)
	return err
}

func runWeb(webDir string) error {
	port := impl.ServerPort
	projPath := impl.ProjectPath
	if !impl.IsFileExist(filepath.Join(projPath, ".builds", "web", "engineres.zip")) {
		exportWeb(webDir)
	}
	impl.StopWebServer()
	scriptPath := filepath.Join(projPath, ".godot", "gdspx_web_server.py")
	scriptPath = strings.ReplaceAll(scriptPath, "\\", "/")
	executeDir := filepath.Join(projPath, ".builds/web")
	executeDir = strings.ReplaceAll(executeDir, "\\", "/")
	println("web server running at http://127.0.0.1:" + fmt.Sprint(port))
	cmd := exec.Command("python", scriptPath, "-r", executeDir, "-p", fmt.Sprint(port))
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting server: %v", err)
	}
	return nil
}

func getISpxPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	targetPath := path.Join(gopath, "bin")
	filePath := path.Join(targetPath, "ispx2.wasm")
	return filePath
}

func installISpx() {
	filePath := getISpxPath()
	rawdir, _ := os.Getwd()
	os.Chdir("../ispx")
	envVars := []string{"GOOS=js", "GOARCH=wasm"}
	impl.RunGolang(envVars, "build", "-o", filePath)
	os.Chdir(rawdir)
}

type DirInfos struct {
	path string
	info os.FileInfo
}

func packProject(baseFolder string, dstZipPath string) {
	paths := []DirInfos{}
	if impl.IsFileExist(dstZipPath) {
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

func showHelpInfo() {
	impl.ShowHelpInfo("spx")
}

func buildDll(project, outputPath string) {
	project, _ = filepath.Abs(project)
	outputPath, _ = filepath.Abs(outputPath)
	spxProjPath, _ := filepath.Abs(project + "/..")
	os.Remove(path.Join(spxProjPath, "gop_autogen.go"))
	os.Remove(path.Join(project, "main.go"))
	rawdir, _ := os.Getwd()
	os.Chdir(spxProjPath)
	envVars := []string{""}
	impl.RunGoplus(envVars, "build", "-o", "gdspx-demo.exe")
	os.Rename(path.Join(spxProjPath, "gop_autogen.go"), path.Join(project, "main.go"))
	os.Remove(path.Join(spxProjPath, "gdspx-demo.exe"))
	os.Chdir(project)
	impl.RunGolang(nil, "mod", "tidy")
	os.Chdir(rawdir)
	impl.BuildDll(project, outputPath)

}

func clearProject() {
	dir := impl.TargetDir
	os.RemoveAll(dir)
	os.Remove(path.Join(dir, "../.gitignore"))
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
	impl.CopyEmbed(engineFiles, "template/project", dstDir)
	println(dstDir)

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
