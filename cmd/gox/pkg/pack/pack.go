package pack

import (
	"archive/zip"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

type DirInfos struct {
	path string
	info os.FileInfo
}

func PackProject(baseFolder string, dstZipPath string) {
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

	PackZip(zipWriter, baseFolder, paths)
}

func PackZip(zipWriter *zip.Writer, baseFolder string, paths []DirInfos) {
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
func SaveEngineHash(webDir string) {
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
	file, err := os.OpenFile(path.Join(webDir, "spxgame.js"), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	if _, err := file.WriteString(js); err != nil {
		panic(err)
	}
}

func PackEngineRes(proejct_fs embed.FS, webDir string) {
	dstDir := path.Join(webDir, "project")
	util.CopyDir(proejct_fs, "template/project", dstDir, true)

	directories := []string{"engine"}
	files := []string{"main.tscn", "project.godot"}
	err := PackDirFiles(path.Join(webDir, "engineres.zip"), dstDir, directories, files)
	if err != nil {
		panic(err)
	}
	os.RemoveAll(dstDir)
}

func PackDirFiles(zipName string, targetDir string, directories, files []string) error {
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

	PackZip(zipWriter, targetDir, paths)
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
