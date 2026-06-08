package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirPreservesExistingFileWhenOverrideDisabled(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()

	srcDir := filepath.Join(srcRoot, "template")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", srcDir, err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "project.godot"), []byte("template"), 0o644); err != nil {
		t.Fatalf("WriteFile(template project.godot) returned error: %v", err)
	}
	dstPath := filepath.Join(dstRoot, "project.godot")
	if err := os.WriteFile(dstPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile(existing project.godot) returned error: %v", err)
	}

	if err := CopyDir(os.DirFS(srcRoot), "template", dstRoot, false); err != nil {
		t.Fatalf("CopyDir(..., false) returned error: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", dstPath, err)
	}
	if string(got) != "existing" {
		t.Fatalf("project.godot = %q, want existing", string(got))
	}
}

func TestCopyDirOverwritesExistingFileWhenOverrideEnabled(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()

	srcDir := filepath.Join(srcRoot, "template")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", srcDir, err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "project.godot"), []byte("template"), 0o644); err != nil {
		t.Fatalf("WriteFile(template project.godot) returned error: %v", err)
	}
	dstPath := filepath.Join(dstRoot, "project.godot")
	if err := os.WriteFile(dstPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile(existing project.godot) returned error: %v", err)
	}

	if err := CopyDir(os.DirFS(srcRoot), "template", dstRoot, true); err != nil {
		t.Fatalf("CopyDir(..., true) returned error: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", dstPath, err)
	}
	if string(got) != "template" {
		t.Fatalf("project.godot = %q, want template", string(got))
	}
}

func TestCopyDirCopiesDotGitignoreFile(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()

	srcDir := filepath.Join(srcRoot, "template")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", srcDir, err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".gitignore"), []byte("/.godot\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(template .gitignore) returned error: %v", err)
	}

	if err := CopyDir(os.DirFS(srcRoot), "template", dstRoot, true); err != nil {
		t.Fatalf("CopyDir(..., true) returned error: %v", err)
	}

	dstPath := filepath.Join(dstRoot, ".gitignore")
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", dstPath, err)
	}
	if string(got) != "/.godot\n" {
		t.Fatalf(".gitignore = %q, want /.godot\\n", string(got))
	}
}

func TestCopyDirGitignoreTemplateOverridesRepoLocalGitignore(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()

	srcDir := filepath.Join(srcRoot, "template")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", srcDir, err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".gitignore"), []byte("/.godot\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(repo-local .gitignore) returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".gitignore.txt"), []byte("/engine\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(template .gitignore.txt) returned error: %v", err)
	}

	if err := CopyDir(os.DirFS(srcRoot), "template", dstRoot, true); err != nil {
		t.Fatalf("CopyDir(..., true) returned error: %v", err)
	}

	dstPath := filepath.Join(dstRoot, ".gitignore")
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", dstPath, err)
	}
	if string(got) != "/engine\n" {
		t.Fatalf(".gitignore = %q, want /engine\\n", string(got))
	}
}
