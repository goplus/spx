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

package release

import (
	"bytes"
	"strings"
	"testing"
)

func TestProjectGodotPreset(t *testing.T) {
	content := []byte(`[preset.0]
name="Android"
runnable=true

[preset.0.options]
architectures/arm64-v8a=true

[preset.7]
name="Linux"
runnable=true

[preset.7.options]
custom_template/debug=""
binary_format/architecture="x86_64"
`)
	got, err := projectGodotPreset(content, "Linux")
	if err != nil {
		t.Fatal(err)
	}
	want := "[preset]\nname=\"Linux\"\nrunnable=true\n[preset.options]\ncustom_template/debug=\"\"\nbinary_format/architecture=\"x86_64\"\n"
	if string(got) != want {
		t.Fatalf("projectGodotPreset() = %q, want %q", got, want)
	}
}

func TestProjectGodotPresetRejectsMissingOrDuplicate(t *testing.T) {
	for _, content := range []string{
		"[preset.0]\nname=\"Web\"\n",
		"[preset.0]\nname=\"Linux\"\n[preset.0.options]\na=1\n[preset.1]\nname=\"Linux\"\n[preset.1.options]\na=2\n",
	} {
		if _, err := projectGodotPreset([]byte(content), "Linux"); err == nil {
			t.Fatalf("projectGodotPreset(%q) succeeded", content)
		}
	}
}

func TestProjectGodotPresetIgnoresOtherPresets(t *testing.T) {
	content := `[preset.0]
name="Android"
[preset.0.options]
version/code=1
[preset.1]
name="Linux"
[preset.1.options]
binary_format/architecture="x86_64"
`
	baseline, err := projectGodotPreset([]byte(content), "Linux")
	if err != nil {
		t.Fatal(err)
	}
	android, err := projectGodotPreset([]byte(strings.Replace(content, "version/code=1", "version/code=2", 1)), "Linux")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(android, baseline) {
		t.Fatal("Android-only change affected Linux preset")
	}
	linux, err := projectGodotPreset([]byte(strings.Replace(content, "x86_64", "arm64", 1)), "Linux")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(linux, baseline) {
		t.Fatal("Linux change did not affect projected preset")
	}
}
