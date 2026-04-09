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

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGoModUsesExplicitVersionFromFixedTemplate(t *testing.T) {
	testEnsureGoModUsesExplicitVersion(t, `module github.com/goplus/spxdemo

go 1.25.0

require github.com/goplus/spx/v2 v0.0.0-test //xgo:class
`)
}

func TestEnsureGoModUsesExplicitVersionFromCRLFTemplate(t *testing.T) {
	testEnsureGoModUsesExplicitVersion(t, "module github.com/goplus/spxdemo\r\n\r\ngo 1.25.0\r\n\r\nrequire github.com/goplus/spx/v2 v0.0.0-test //xgo:class\r\n")
}

func testEnsureGoModUsesExplicitVersion(t *testing.T, template string) {
	t.Helper()

	oldTemplate := GoModTemplate
	GoModTemplate = template
	t.Cleanup(func() {
		GoModTemplate = oldTemplate
	})

	projectDir := t.TempDir()
	r := &Runner{
		ProjectDir:    projectDir,
		RunnerVersion: "v9.9.9-test",
	}

	if err := r.ensureGoMod(); err != nil {
		t.Fatalf("ensureGoMod failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "module "+filepath.Base(projectDir)) {
		t.Fatalf("go.mod should use project directory as module name, got:\n%s", content)
	}

	expectedRequire := "require " + SpxModule + " v9.9.9-test //xgo:class"
	if !strings.Contains(content, expectedRequire) {
		t.Fatalf("go.mod should pin requested spx version %q, got:\n%s", expectedRequire, content)
	}

	if strings.Contains(template, "\r\n") && !strings.Contains(content, expectedRequire+"\r\n") {
		t.Fatalf("go.mod should preserve CRLF line endings for %q, got:\n%s", expectedRequire, content)
	}
}
