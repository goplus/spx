package shared

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goplus/spx/v2/internal/releasemeta"
)

func ensureGoPath() (string, error) {
	if goPath := os.Getenv("GOPATH"); goPath != "" {
		return goPath, nil
	}

	output, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return "", err
	}
	goPath := strings.TrimSpace(string(output))
	if goPath == "" {
		return "", fmt.Errorf("missing GOPATH")
	}
	return goPath, nil
}

func defaultRuntimeVersion() (string, error) {
	version := releasemeta.DefaultReleaseMeta().Runtime.Version
	if version == "" {
		return "", fmt.Errorf("releasemeta: Runtime.Version is empty")
	}
	return version, nil
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	info, err := input.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func writeNamedZip(dst string, namedFiles map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return err
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)

	names := make([]string, 0, len(namedFiles))
	for name := range namedFiles {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		src := namedFiles[name]
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate

		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(src)
		if err != nil {
			return err
		}
		if _, err := io.Copy(entry, input); err != nil {
			input.Close()
			return err
		}
		input.Close()
	}
	return writer.Close()
}

func zipDirectory(srcDir, dstZip string) error {
	if !fileExists(srcDir) {
		return fmt.Errorf("source directory does not exist: %s", srcDir)
	}
	if err := os.MkdirAll(filepath.Dir(dstZip), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dstZip); err != nil {
		return err
	}

	var files []string
	if err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(files)

	file, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)

	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate

		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(entry, input); err != nil {
			input.Close()
			return err
		}
		input.Close()
	}
	return writer.Close()
}

func extractZip(srcZip, dstDir string) error {
	reader, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath, err := resolveZipExtractPath(dstDir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(file, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func resolveZipExtractPath(dstDir, name string) (string, error) {
	cleanBase := filepath.Clean(dstDir)
	targetPath := filepath.Clean(filepath.Join(cleanBase, name))
	basePrefix := cleanBase
	if !strings.HasSuffix(basePrefix, string(os.PathSeparator)) {
		basePrefix += string(os.PathSeparator)
	}
	targetPrefix := targetPath
	if !strings.HasSuffix(targetPrefix, string(os.PathSeparator)) {
		targetPrefix += string(os.PathSeparator)
	}
	if targetPath != cleanBase && !strings.HasPrefix(targetPrefix, basePrefix) {
		return "", fmt.Errorf("illegal path in archive entry: %s", name)
	}
	return targetPath, nil
}

func extractZipFile(file *zip.File, dst string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, reader)
	return err
}

func fetchURLToFile(url, dst string) (err error) {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s failed: %s", url, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	defer func() {
		if file != nil {
			_ = file.Close()
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	file = nil

	return os.Rename(tmpPath, dst)
}
