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
	"errors"
	"strings"
	"testing"
)

func TestRuntimeManifestPinsMatchLocks(t *testing.T) {
	if len(runtimeManifestPins) == 0 {
		t.Fatal("no runtime manifest pins")
	}
	for version := range runtimeManifestPins {
		t.Run(version, func(t *testing.T) {
			lock, err := RuntimeLockForVersion(version)
			if err != nil {
				t.Fatal(err)
			}
			pin, err := RuntimeManifestPinForLock(lock)
			if err != nil {
				t.Fatal(err)
			}
			if pin.RuntimeVersion != lock.RuntimeVersion || pin.Name != lock.Manifest || pin.Size <= 0 || len(pin.SHA256) != 64 {
				t.Fatalf("runtime manifest pin = %#v", pin)
			}
		})
	}
}

func TestRuntimeManifestPinValidation(t *testing.T) {
	lock, err := RuntimeLockForVersion("2.4.1")
	if err != nil {
		t.Fatal(err)
	}
	pin, err := RuntimeManifestPinForLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*RuntimeManifestPin){
		func(p *RuntimeManifestPin) { p.Schema++ },
		func(p *RuntimeManifestPin) { p.RuntimeVersion = "invalid" },
		func(p *RuntimeManifestPin) { p.Name = "../manifest.json" },
		func(p *RuntimeManifestPin) { p.Size = 0 },
		func(p *RuntimeManifestPin) { p.SHA256 = strings.Repeat("A", 64) },
	} {
		candidate := pin
		mutate(&candidate)
		if err := candidate.validate(); err == nil {
			t.Fatalf("invalid runtime manifest pin accepted: %#v", candidate)
		}
	}
}

func TestRuntimeManifestPinForLockRejectsUnpinnedRuntime(t *testing.T) {
	lock := DefaultRuntimeLock()
	lock.RuntimeVersion = "9.9.9"
	if _, err := RuntimeManifestPinForLock(lock); err == nil || !errors.Is(err, ErrRuntimeManifestPinNotFound) || !strings.Contains(err.Error(), "no runtime manifest pin") {
		t.Fatalf("RuntimeManifestPinForLock error = %v", err)
	}
}

func TestRuntimeManifestPinForLockDoesNotInventCurrentRuntime(t *testing.T) {
	lock := DefaultRuntimeLock()
	if lock.RuntimeVersion != "2.4.4" {
		t.Fatalf("default runtime version = %q, want current unpublished 2.4.4", lock.RuntimeVersion)
	}
	if _, ok := runtimeManifestPins[lock.RuntimeVersion]; ok {
		t.Fatalf("runtime manifest pin unexpectedly exists for unpublished %s", lock.RuntimeVersion)
	}
	if _, err := RuntimeManifestPinForLock(lock); err == nil || !strings.Contains(err.Error(), "no runtime manifest pin") {
		t.Fatalf("RuntimeManifestPinForLock error = %v, want missing-pin failure", err)
	}
}
