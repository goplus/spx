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
	"strings"
	"testing"

	"github.com/goplus/ixgo"
)

func TestConfigureFilesystemRootsRejectsAfterInit(t *testing.T) {
	mu.Lock()
	previous := ixgoCtx
	ixgoCtx = &ixgo.Context{}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		ixgoCtx = previous
		mu.Unlock()
	})

	err := ConfigureFilesystemRoots("/unused/project", "/unused/assets")
	if err == nil || !strings.Contains(err.Error(), "before Init") {
		t.Fatalf("ConfigureFilesystemRoots() error = %v, want lifecycle rejection", err)
	}
}
