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

package ispx

import (
	"os"
	"path/filepath"
	"testing"
)

// Keep the complete examples used by the spx-project skill buildable.
// Building does not launch Godot or validate visual and audio behavior.
func TestProjectTutorials(t *testing.T) {
	for _, name := range []string{"01-Weather", "02-Dragon", "03-Clone"} {
		t.Run(name, func(t *testing.T) {
			isolateInterpreter(t)
			if err := Init(nil); err != nil {
				t.Fatal(err)
			}
			if err := BuildFS(os.DirFS(filepath.Join("..", "..", "tutorial", name))); err != nil {
				t.Fatal(err)
			}
		})
	}
}
