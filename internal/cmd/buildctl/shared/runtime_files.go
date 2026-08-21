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

package shared

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/goplus/spx/v3/internal/base/fileutil"
	"github.com/goplus/spx/v3/internal/release"
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
	return release.DefaultRuntimeLock().RuntimeVersion, nil
}

func copyFile(src, dst string) error {
	return fileutil.CopyFile(src, dst)
}

func writeNamedZip(dst string, namedFiles map[string]string) error {
	return fileutil.WriteNamedZip(dst, namedFiles)
}
