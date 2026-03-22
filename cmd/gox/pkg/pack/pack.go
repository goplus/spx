package pack

import (
	"archive/zip"
	"embed"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/goplus/spx/v2/cmd/gox/pkg/util"
)

type DirInfos struct {
	path string
	info os.FileInfo
	// zipPath optionally overrides the archive entry path for assets outside baseFolder.
	zipPath string
}

func PackProject(baseFolder string, dstZipPath string) error {
	paths := []DirInfos{}
	if util.IsFileExist(dstZipPath) {
		if err := os.Remove(dstZipPath); err != nil {
			return err
		}
	}
	skipDirs := map[string]struct{}{
		".git": {}, "project": {},
	}

	file, err := os.Create(dstZipPath)
	if err != nil {
		return err
	}
	zipWriter := zip.NewWriter(file)
	closeZip := func(err error) error {
		if closeErr := zipWriter.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		return err
	}

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
		paths = append(paths, DirInfos{path: path, info: info})
		return nil
	})
	if err != nil {
		return closeZip(err)
	}

	existingZipPaths := make(map[string]struct{}, len(paths))
	for _, dirInfo := range paths {
		existingZipPaths[zipEntryName(baseFolder, dirInfo)] = struct{}{}
	}

	extraPaths, err := collectExternalAssetPaths(baseFolder, existingZipPaths)
	if err != nil {
		return closeZip(err)
	}
	paths = append(paths, extraPaths...)

	return closeZip(PackZip(zipWriter, baseFolder, paths))
}

func PackZip(zipWriter *zip.Writer, baseFolder string, paths []DirInfos) error {
	baseFolder = strings.ReplaceAll(baseFolder, "\\", "/")
	slices.SortFunc(paths, func(a, b DirInfos) int {
		nameA := zipEntryName(baseFolder, a)
		nameB := zipEntryName(baseFolder, b)
		if nameA < nameB {
			return -1
		} else if nameA > nameB {
			return 1
		}
		return 0
	})
	for _, dirInfo := range paths {
		path := dirInfo.path
		path = strings.ReplaceAll(path, "\\", "/")
		info := dirInfo.info
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		// Set a fixed timestamp
		header.Modified = time.Unix(0, 0)

		header.Name = zipEntryName(baseFolder, dirInfo)
		if header.Name == "" {
			continue
		}
		if info.IsDir() {
			header.Name += "/"
			_, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}
			continue
		}

		fileToZip, err := os.Open(path)
		if err != nil {
			return err
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			fileToZip.Close()
			return err
		}
		_, copyErr := io.Copy(writer, fileToZip)
		closeErr := fileToZip.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func zipEntryName(baseFolder string, dirInfo DirInfos) string {
	if dirInfo.zipPath != "" {
		return strings.TrimPrefix(normalizeZipPath(dirInfo.zipPath), "/")
	}

	baseFolder = normalizeZipPath(baseFolder)
	name := strings.TrimPrefix(normalizeZipPath(dirInfo.path), baseFolder)
	return strings.TrimPrefix(name, "/")
}

func normalizeZipPath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
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
	zipWriter := zip.NewWriter(zipFile)
	closeZip := func(err error) error {
		if closeErr := zipWriter.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := zipFile.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		return err
	}

	paths := []DirInfos{}
	for _, dir := range directories {
		paths, err = addDirToZip(path.Join(targetDir, dir), paths)
		if err != nil {
			return closeZip(err)
		}
	}

	for _, file := range files {
		paths, err = addFileToZip(path.Join(targetDir, file), paths)
		if err != nil {
			return closeZip(err)
		}
	}

	return closeZip(PackZip(zipWriter, targetDir, paths))
}

func addDirToZip(dirPath string, paths []DirInfos) ([]DirInfos, error) {
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, DirInfos{path: path, info: info})
		return nil
	})
	return paths, err
}

func addFileToZip(path string, paths []DirInfos) ([]DirInfos, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	paths = append(paths, DirInfos{path: path, info: info})
	return paths, nil
}
