package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSConsScriptIncludesCommonArgs(t *testing.T) {
	script := sconsScript([]string{"platform=android target=template_debug arch=arm32"})
	if !strings.Contains(script, "scons optimize=size") {
		t.Fatalf("expected common args in script: %s", script)
	}
	if !strings.Contains(script, "platform=android target=template_debug arch=arm32") {
		t.Fatalf("expected command args in script: %s", script)
	}
}

func TestMergeStringMapsOverridesExisting(t *testing.T) {
	merged := mergeStringMaps(
		map[string]string{"PATH": "/usr/bin", "A": "1"},
		map[string]string{"PATH": "/custom/bin:/usr/bin", "B": "2"},
	)
	if merged["PATH"] != "/custom/bin:/usr/bin" || merged["A"] != "1" || merged["B"] != "2" {
		t.Fatalf("unexpected merged map: %#v", merged)
	}
}

func TestPopulateWebTemplateCopies(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "web_dlink_debug.zip")
	mustWriteFile(t, src, []byte("zip"))
	mustWriteFile(t, filepath.Join(root, "web_old.zip"), []byte("old"))

	if err := populateWebTemplateCopies(src, root); err != nil {
		t.Fatalf("populateWebTemplateCopies returned error: %v", err)
	}
	if fileExists(filepath.Join(root, "web_old.zip")) {
		t.Fatal("expected old web zip to be removed")
	}
	for _, name := range []string{
		"web_dlink_nothreads_debug.zip",
		"web_dlink_nothreads_release.zip",
		"web_nothreads_debug.zip",
		"web_nothreads_release.zip",
		"web_dlink_debug.zip",
		"web_dlink_release.zip",
		"web_debug.zip",
		"web_release.zip",
	} {
		if !fileExists(filepath.Join(root, name)) {
			t.Fatalf("expected %s to exist", name)
		}
	}
}
