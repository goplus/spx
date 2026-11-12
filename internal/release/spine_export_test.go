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

package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestSpineNoticeExport checks the real export pipeline when an editor is
// supplied. Read the resulting pack from a different project so missing PCK
// entries cannot accidentally resolve from the source directory.
func TestSpineNoticeExport(t *testing.T) {
	editor := os.Getenv("SPX_SPINE_GODOT")
	if editor == "" {
		t.Skip("set SPX_SPINE_GODOT to a locally built Godot editor")
	}
	root := t.TempDir()
	project, reader := filepath.Join(root, "project"), filepath.Join(root, "reader")
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	configuration := []byte("config_version=5\n[application]\nconfig/name=\"Spine notice export\"\n")
	write(filepath.Join(project, "project.godot"), configuration)
	write(filepath.Join(reader, "project.godot"), configuration)
	template := filepath.Join("..", "..", "cmd", "spx", "template", "project")
	presets, err := os.ReadFile(filepath.Join(template, "export_presets.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(project, "export_presets.cfg"), presets)
	pack := filepath.Join(root, "notices.pck")
	script := "extends SceneTree\nfunc _initialize():\n"
	script += fmt.Sprintf("\tassert(ProjectSettings.load_resource_pack(%s))\n", strconv.Quote(filepath.ToSlash(pack)))
	for _, name := range []string{"SpineRuntime.txt", "SpineJson.txt"} {
		data, err := os.ReadFile(filepath.Join(template, "engine", "licenses", name))
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(project, "engine", "licenses", name), data)
		script += fmt.Sprintf("\tassert(FileAccess.get_sha256(\"res://engine/licenses/%s\") == \"%x\")\n", name, sha256.Sum256(data))
	}
	script += "\tprint(\"SPINE_NOTICES_PCK_OK\")\n\tquit()\n"
	write(filepath.Join(reader, "read.gd"), []byte(script))
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, editor, args...)
		cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+filepath.Join(root, "cache"), "XDG_DATA_HOME="+filepath.Join(root, "data"), "XDG_CONFIG_HOME="+filepath.Join(root, "config"))
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Godot %v: %v\n%s", args, err, output)
		}
		return output
	}
	run("--headless", "--editor", "--quit", "--path", project, "--import")
	run("--headless", "--path", project, "--export-pack", "Linux", pack)
	output := run("--headless", "--path", reader, "--script", "read.gd")
	if !bytes.Contains(output, []byte("SPINE_NOTICES_PCK_OK")) {
		t.Fatalf("notice verification did not complete:\n%s", output)
	}
}
