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

package command

import (
	"embed"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCmdPropagatesSpecialCommandErrors(t *testing.T) {
	for _, name := range []string{"clear", "clearbuild", "stopweb"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			projectDir := "project"
			if name == "stopweb" {
				cmd := &CmdTool{TargetAbsDir: root}
				if err := os.Mkdir(cmd.webServerPIDPath(), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				// Removal must fail for an overlong path component on all hosts.
				if err := os.Mkdir(projectDir, 0o755); err != nil {
					t.Fatal(err)
				}
				projectDir = filepath.Join(projectDir, strings.Repeat("x", 300))
			}
			oldFlags, oldArgs := flag.CommandLine, os.Args
			flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = []string{"spx", name, "--path", root}
			t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })

			cmd := &CmdTool{}
			err := cmd.RunCmd("spx", ".spx", "test", embed.FS{}, "", projectDir)
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("RunCmd(%s) error = %v, want filesystem error", name, err)
			}
		})
	}
}
