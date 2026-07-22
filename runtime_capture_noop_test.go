//go:build !js

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

package spx

import (
	"testing"

	"github.com/goplus/spx/v2/internal/engine"
)

func TestCaptureDefaultsToNoopHandlerOnNative(t *testing.T) {
	engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()

	engine.SetCaptureHandler(ignoreCaptureRequest)

	if err := engine.EnqueueCapture("step_001.png"); err != nil {
		t.Fatalf("enqueueCapture returned error on native platform: %v", err)
	}
}
