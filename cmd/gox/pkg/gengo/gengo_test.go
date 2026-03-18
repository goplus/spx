package gengo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenGoFromFS(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{name: "PreferredSourceExt", filename: preferredMainFile},
		{name: "LegacySourceExt", filename: legacyMainFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "main.go")

			source := "onStart => {\n}\n"
			if err := os.WriteFile(filepath.Join(dir, tt.filename), []byte(source), 0644); err != nil {
				t.Fatalf("write source file: %v", err)
			}

			if err := GenGoFromFS(NewDirFS(dir), outputPath); err != nil {
				t.Fatalf("GenGoFromFS: %v", err)
			}

			output, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("read generated file: %v", err)
			}
			if !strings.Contains(string(output), "package main") {
				t.Fatalf("generated file does not contain package main:\n%s", output)
			}
		})
	}
}
