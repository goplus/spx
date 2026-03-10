package webhost

import (
	"io/fs"
	"strings"
	"testing"
)

func TestAssetsContainDeclaredFiles(t *testing.T) {
	expectedFiles := []string{
		"runner.html",
		"game.js",
		"worker.message.manager.js",
	}
	for _, name := range expectedFiles {
		if _, err := fs.ReadFile(Assets, name); err != nil {
			t.Fatalf("read embedded host asset %q: %v", name, err)
		}
	}
}

func TestRunnerHTMLDoesNotContainBuilderSpecificHooks(t *testing.T) {
	content, err := fs.ReadFile(Assets, "runner.html")
	if err != nil {
		t.Fatalf("read runner.html: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "spx_set_ai_") {
		t.Fatal("runner.html should not expose generic AI bridge hooks")
	}
	if strings.Contains(text, "xbuilder_set_ai_") {
		t.Fatal("runner.html should not expose builder-specific AI bridge hooks")
	}
}

func TestGameJSInstantiatesWorkerManagerOnlyInWorkerMode(t *testing.T) {
	content, err := fs.ReadFile(Assets, "game.js")
	if err != nil {
		t.Fatalf("read game.js: %v", err)
	}
	if !strings.Contains(string(content), "this.workerMode ? new globalThis.WorkerMessageManager() : null") {
		t.Fatal("game.js should construct WorkerMessageManager only in worker mode")
	}
}
