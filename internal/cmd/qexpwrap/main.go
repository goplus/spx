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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/base/licenseheader"
)

func main() {
	args := os.Args[1:]
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
