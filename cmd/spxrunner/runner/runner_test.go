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

func TestEnsureGoModUsesExplicitVersionFromPlaceholderTemplate(t *testing.T) {
	testEnsureGoModUsesExplicitVersion(t, `module github.com/goplus/spxdemo

go 1.25.0

require github.com/goplus/spx/v2 {{SPX_VERSION}} //xgo:class
`)
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

	if strings.Contains(content, "{{SPX_VERSION}}") {
		t.Fatalf("go.mod should not keep SPX_VERSION placeholder, got:\n%s", content)
	}
}
