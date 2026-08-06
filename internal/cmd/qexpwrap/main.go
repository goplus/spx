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
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goplus/spx/v3/internal/base/licenseheader"
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
