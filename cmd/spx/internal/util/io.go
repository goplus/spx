package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var errFileFound = errors.New("file found")

// CopyDir2 copies a directory from the local filesystem.
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

// CheckFileExist reports whether dir contains a file with ext.
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
				return errFileFound
			}
			return nil
		})

		if errors.Is(err, errFileFound) {
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

// IsFileExist reports whether path exists.
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

// CopyDir copies a directory from fsys into dstDir.
func CopyDir(fsys fs.FS, srcDir, dstDir string, isOverride bool) error {
	subfs, err := fs.Sub(fsys, srcDir)
	if err != nil {
		return fmt.Errorf("create sub fs for %s: %w", srcDir, err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dstDir, err)
	}
	return fs.WalkDir(subfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk directory %s: %w", srcDir, err)
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
