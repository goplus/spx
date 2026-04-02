package util

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func CopyDir2(src string, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

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
			err = CopyDir2(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func CheckFileExist(dir, ext string, recursive bool) bool {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	if recursive {
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
func IsFileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CopyFile copies a file.
func CopyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, input, 0755); err != nil {
		return err
	}
	return nil
}

func CopyDir(fsys fs.FS, srcDir, dstDir string, isOverride bool) error {
	subfs, err := fs.Sub(fsys, srcDir)
	if err != nil {
		println("Error: create sub fs: ", srcDir, dstDir)
		return err
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		println("Error: creating directory: ", dstDir)
		return err
	}
	return fs.WalkDir(subfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			println("Error: walking directory: ", srcDir)
			return err
		}

		dstPath := filepath.Join(dstDir, path)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		} else {
			if strings.HasSuffix(dstPath, "go.mod.txt") {
				i := strings.LastIndex(dstPath, "go.mod.txt")
				dstPath = dstPath[:i] + "go.mod"
			}
			if strings.HasSuffix(dstPath, ".gitignore.txt") {
				i := strings.LastIndex(dstPath, ".gitignore.txt")
				dstPath = dstPath[:i] + ".gitignore"
			}

			if !isOverride {
				if _, err := os.Stat(dstPath); !os.IsNotExist(err) {
					return nil
				}
			}

			srcFile, err := subfs.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			return err
		}
	})
}
