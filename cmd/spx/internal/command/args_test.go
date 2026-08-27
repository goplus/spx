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
	"flag"
	"io"
	"os"
	"testing"
)

func TestParseCommandLineArgsRejectsRemovedGoEnv(t *testing.T) {
	originalFlags := flag.CommandLine
	originalArgs := os.Args
	flag.CommandLine = flag.NewFlagSet("spx", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"spx", "run", "--goenv", "/tmp/goenv"}
	t.Cleanup(func() {
		flag.CommandLine = originalFlags
		os.Args = originalArgs
	})

	cmd := &CmdTool{}
	help := cmd.initializeFlags()
	if err := cmd.parseCommandLineArgs(help); err == nil {
		t.Fatal("legacy --goenv flag was accepted")
	}
}

func TestCheckCmdAcceptsInternalExportPack(t *testing.T) {
	cmd := &CmdTool{Args: ExtraArgs{CmdName: "exportpack"}}
	if !cmd.CheckCmd() {
		t.Fatal("internal exportpack command was rejected")
	}
}
