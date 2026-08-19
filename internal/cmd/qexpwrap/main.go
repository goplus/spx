/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/base/licenseheader"
	"golang.org/x/tools/go/gcexportdata"
)

func main() {
	args, err := expandDirectCallsFile(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cmd := exec.Command("go", append([]string{"tool", "qexp"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		code := 1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		os.Exit(code)
	}

	outdir, ok := flagValue(args, "-outdir")
	if !ok || outdir == "" {
		return
	}
	if err := canonicalizeTypesInDir(outdir); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
	if err := addHeadersInDir(outdir); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func expandDirectCallsFile(args []string) ([]string, error) {
	const fileFlag = "-directcalls-file"
	fileIndex := -1
	filePath := ""
	separateValue := false
	directCalls := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		switch {
		case arg == fileFlag:
			if fileIndex >= 0 {
				return nil, fmt.Errorf("%s may only be specified once", fileFlag)
			}
			if i+1 >= len(args) {
				return nil, fmt.Errorf("%s requires a path", fileFlag)
			}
			fileIndex = i
			filePath = args[i+1]
			separateValue = true
			i++
		case strings.HasPrefix(arg, fileFlag+"="):
			if fileIndex >= 0 {
				return nil, fmt.Errorf("%s may only be specified once", fileFlag)
			}
			fileIndex = i
			filePath = strings.TrimPrefix(arg, fileFlag+"=")
		case arg == "-directcalls":
			directCalls = true
			if i+1 < len(args) {
				i++
			}
		case strings.HasPrefix(arg, "-directcalls="):
			directCalls = true
		}
	}
	if fileIndex < 0 {
		return args, nil
	}
	if directCalls {
		return nil, fmt.Errorf("%s cannot be combined with -directcalls", fileFlag)
	}
	if filePath == "" {
		return nil, fmt.Errorf("%s requires a path", fileFlag)
	}
	selectors, err := readDirectCallSelectors(filePath)
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", fileFlag, filePath, err)
	}

	ret := append([]string(nil), args...)
	if separateValue {
		ret[fileIndex] = "-directcalls"
		ret[fileIndex+1] = selectors
	} else {
		ret[fileIndex] = "-directcalls=" + selectors
	}
	return ret, nil
}

func readDirectCallSelectors(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var selectors []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		if selector := strings.TrimSpace(line); selector != "" {
			selectors = append(selectors, selector)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(selectors) == 0 {
		return "", fmt.Errorf("contains no selectors")
	}
	return strings.Join(selectors, ","), nil
}

func addHeadersInDir(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		return licenseheader.AddToGoFile(path)
	})
}

type sourceRoots struct {
	moduleRoot  string
	goRoot      string
	moduleCache string
}

func canonicalizeTypesInDir(root string) error {
	var files []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".types" {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	roots, err := loadSourceRoots()
	if err != nil {
		return err
	}
	for _, file := range files {
		rel, err := filepath.Rel(root, filepath.Dir(file))
		if err != nil {
			return err
		}
		if err := canonicalizeTypesFile(file, filepath.ToSlash(rel), roots); err != nil {
			return fmt.Errorf("canonicalize %s: %w", file, err)
		}
	}
	return nil
}

func loadSourceRoots() (sourceRoots, error) {
	cmd := exec.Command("go", "env", "-json", "GOMOD", "GOROOT", "GOMODCACHE")
	data, err := cmd.Output()
	if err != nil {
		return sourceRoots{}, fmt.Errorf("load Go source roots: %w", err)
	}
	var env struct {
		GoMod      string `json:"GOMOD"`
		GoRoot     string `json:"GOROOT"`
		GoModCache string `json:"GOMODCACHE"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return sourceRoots{}, fmt.Errorf("decode Go source roots: %w", err)
	}
	return sourceRoots{
		moduleRoot:  filepath.Dir(env.GoMod),
		goRoot:      env.GoRoot,
		moduleCache: env.GoModCache,
	}, nil
}

func canonicalizeTypesFile(filename, pkgPath string, roots sourceRoots) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	data, err = canonicalizeTypesData(data, pkgPath, roots)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0o666)
}

func canonicalizeTypesData(data []byte, pkgPath string, roots sourceRoots) ([]byte, error) {
	fset := token.NewFileSet()
	pkg, err := gcexportdata.Read(bytes.NewReader(data), fset, make(map[string]*types.Package), pkgPath)
	if err != nil {
		return nil, err
	}

	canonicalFset := token.NewFileSet()
	fset.Iterate(func(file *token.File) bool {
		canonicalFile := canonicalFset.AddFile(canonicalSourcePath(file.Name(), roots), file.Base(), file.Size())
		canonicalFile.SetLines(file.Lines())
		return true
	})

	var buf bytes.Buffer
	if err := gcexportdata.Write(&buf, canonicalFset, pkg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func canonicalSourcePath(filename string, roots sourceRoots) string {
	for _, candidate := range []struct {
		root   string
		prefix string
	}{
		{roots.moduleRoot, "$MODULE"},
		{roots.goRoot, "$GOROOT"},
		{roots.moduleCache, "$GOMODCACHE"},
	} {
		if rel, ok := relativeTo(candidate.root, filename); ok {
			return candidate.prefix + "/" + filepath.ToSlash(rel)
		}
	}
	if filepath.IsAbs(filename) {
		return "$ABS/" + filepath.Base(filename)
	}
	return filepath.ToSlash(filename)
}

func relativeTo(root, filename string) (string, bool) {
	if root == "" || filename == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, filename)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func flagValue(args []string, name string) (string, bool) {
	for i := range args {
		arg := args[i]
		if arg == name {
			if i+1 >= len(args) {
				return "", false
			}
			return args[i+1], true
		}
		prefix := name + "="
		if len(arg) > len(prefix) && arg[:len(prefix)] == prefix {
			return arg[len(prefix):], true
		}
	}
	return "", false
}
