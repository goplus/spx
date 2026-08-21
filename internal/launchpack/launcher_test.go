/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package launchpack

import "testing"

func TestGeneratedLauncherGolden(t *testing.T) {
	const want = `package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/goplus/spx/v3/x/xgolauncher"
)

//go:embed payload.spxpkg
var payload []byte

const payloadSHA256 = "payload-digest"
const manifestSHA256 = "manifest-digest"

func main() {
	status, err := xgolauncher.RunCommand(context.Background(), func(ctx context.Context) (xgolauncher.ProcessStatus, error) {
		return xgolauncher.RunContext(ctx, xgolauncher.Config{
			Payload: payload, PayloadSHA256: payloadSHA256, ManifestSHA256: manifestSHA256,
			Args: os.Args[1:], Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		})
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "xgolauncher: %v\n", err)
		status = xgolauncher.ProcessStatus{Code: 1}
	}
	xgolauncher.Exit(status)
}
`
	if got := string(renderGeneratedLauncher("payload-digest", "manifest-digest")); got != want {
		t.Fatalf("generated launcher changed:\n%s", got)
	}
}
