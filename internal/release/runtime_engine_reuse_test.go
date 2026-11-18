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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInheritedEngineProvenanceAndBytes(t *testing.T) {
	lock, provenance, inputs, dir := runtimeManifestFixture(t)
	provenance.EngineSourceSHA256 = strings.Repeat("e", 64)
	source, err := GenerateRuntimeManifest(lock, provenance, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err := source.VerifyEngineFiles(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, RuntimePackAssetName)); err != nil {
		t.Fatal(err)
	}
	if err := source.VerifyEngineFiles(dir); err != nil {
		t.Fatalf("obsolete pack required: %v", err)
	}
	target := source
	target.RuntimeVersion = "99.0.0"
	target.Provenance.SPXCommit = strings.Repeat("f", 40)
	if err := target.InheritEngine(source); err != nil {
		t.Fatal(err)
	}
	if target.Provenance.EngineOrigin.SPXCommit != source.Provenance.SPXCommit || target.Provenance.EngineOrigin.RuntimeVersion != source.RuntimeVersion {
		t.Fatal("lost original engine provenance")
	}
	next := target
	next.RuntimeVersion = "99.0.1"
	if err := next.InheritEngine(target); err != nil {
		t.Fatal(err)
	}
	if *next.Provenance.EngineOrigin != *target.Provenance.EngineOrigin {
		t.Fatal("reuse chain lost original build")
	}
	next.Assets = append([]RuntimeAsset(nil), target.Assets...)
	next.Assets[0].SHA256 = strings.Repeat("0", 64)
	if err := next.InheritEngine(target); err == nil {
		t.Fatal("modified copied asset accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, source.Assets[0].Name), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := source.VerifyEngineFiles(dir); err == nil {
		t.Fatal("tampered engine accepted")
	}
}
