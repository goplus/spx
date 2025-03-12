package util

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func CopyDir2(src string, dst string) error {
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
			err = CopyDir2(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// Copy file
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
func IsFileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
func SetupFile(force bool, name, embed string, args ...any) error {
	if _, err := os.Stat(name); force || os.IsNotExist(err) {
		if len(args) > 0 {
			embed = fmt.Sprintf(embed, args...)
		}
		if err := os.WriteFile(name, []byte(embed), 0644); err != nil {
			return err
		}
	}
	return nil
}

// Copy a file
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
			// Skip if file already exists and is not overriden

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

func DownloadFile(url string, dest string) error {
	println("Downloading file... ", url, "=>", dest)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s", resp.Status)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func Unzip(zipfile, dest string) {
	r, err := zip.OpenReader(zipfile)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			panic(err)
		}
		defer rc.Close()

		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			var dir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				dir = fpath[:lastIndex]
			}

			err = os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				log.Fatal(err)
			}

			_, err = io.Copy(outFile, rc)
			outFile.Close()

			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
