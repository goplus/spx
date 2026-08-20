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

// Command xgodriver implements the SPX side of XGo project driver v1.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/goplus/spx/v3/internal/xgodriver"
	"github.com/goplus/spx/v3/x/xgolauncher"
)

func main() {
	status, err := xgolauncher.RunCommand(context.Background(), func(ctx context.Context) (xgolauncher.ProcessStatus, error) {
		cfg, err := xgodriver.Parse(os.Args[1:])
		if err != nil {
			return xgolauncher.ProcessStatus{Code: 2}, err
		}
		return xgodriver.Execute(ctx, cfg, xgodriver.IO{
			Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Env: os.Environ(),
		})
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, commandErrorMessage(err))
		if status.Success() {
			status = xgolauncher.ProcessStatus{Code: 1}
		}
	}
	xgolauncher.Exit(status)
}

func commandErrorMessage(err error) string {
	const prefix = "xgodriver: "
	message := err.Error()
	for strings.HasPrefix(message, prefix) {
		message = strings.TrimPrefix(message, prefix)
	}
	return prefix + message
}
