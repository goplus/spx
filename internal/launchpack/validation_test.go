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

package launchpack

import "testing"

func TestValidateGraphFlags(t *testing.T) {
	if err := validateGraphFlags([]string{"-trimpath", "-mod=readonly", "-buildvcs=false"}); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"trimpath"}, {"-overlay=/tmp/overlay.json"}, {"-modfile"}} {
		if err := validateGraphFlags(flags); err == nil {
			t.Fatalf("validateGraphFlags(%q) succeeded", flags)
		}
	}
}

func TestValidateBuildFlags(t *testing.T) {
	if err := validateBuildFlags([]string{"-v", "-trimpath=true", "-buildvcs=auto"}); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"-tags=custom"}, {"-v=maybe"}, {"-buildvcs"}} {
		if err := validateBuildFlags(flags); err == nil {
			t.Fatalf("validateBuildFlags(%q) succeeded", flags)
		}
	}
}

func TestValidatePackPath(t *testing.T) {
	for _, test := range []struct {
		name      string
		value     string
		directory bool
		wantErr   bool
	}{
		{name: "nested directory", value: "assets/images", directory: true},
		{name: "plain index", value: "index.json"},
		{name: "backslash", value: `assets\images`, directory: true, wantErr: true},
		{name: "absolute", value: "/tmp/assets", directory: true, wantErr: true},
		{name: "windows absolute", value: "C:/assets", directory: true, wantErr: true},
		{name: "parent escape", value: "../assets", directory: true, wantErr: true},
		{name: "dot component", value: "assets/./images", directory: true, wantErr: true},
		{name: "parent component", value: "assets/../images", directory: true, wantErr: true},
		{name: "nested index", value: "indexes/index.json", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validatePackPath("pack path", test.value, test.directory)
			if (err != nil) != test.wantErr {
				t.Fatalf("validatePackPath(%q, directory=%v) error = %v, wantErr=%v", test.value, test.directory, err, test.wantErr)
			}
		})
	}
}
