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

package event

import "testing"

func TestManagerReset(t *testing.T) {
	var mgr Manager
	mgr.Add(BucketStart, Sink{Owner: "game", Handler: func() {}})
	mgr.Add(BucketTimer, Sink{Owner: "sprite", Handler: func() {}})

	mgr.Reset()

	if got := mgr.Snapshot(BucketStart); len(got) != 0 {
		t.Fatalf("BucketStart len = %d, want 0", len(got))
	}
	if got := mgr.Snapshot(BucketTimer); len(got) != 0 {
		t.Fatalf("BucketTimer len = %d, want 0", len(got))
	}
}

func TestManagerDeleteOwner(t *testing.T) {
	var mgr Manager
	mgr.Add(BucketStart, Sink{Owner: "keep", Handler: func() {}})
	mgr.Add(BucketClick, Sink{Owner: "drop", Handler: func() {}})
	mgr.Add(BucketTimer, Sink{Owner: "drop", Handler: func() {}})

	mgr.DeleteOwner("drop")

	if got := mgr.Snapshot(BucketStart); len(got) != 1 {
		t.Fatalf("BucketStart len = %d, want 1", len(got))
	}
	if got := mgr.Snapshot(BucketClick); len(got) != 0 {
		t.Fatalf("BucketClick len = %d, want 0", len(got))
	}
	if got := mgr.Snapshot(BucketTimer); len(got) != 0 {
		t.Fatalf("BucketTimer len = %d, want 0", len(got))
	}
}

func TestManagerSnapshotIsStableAcrossWrites(t *testing.T) {
	var mgr Manager
	mgr.Add(BucketStart, Sink{Owner: "game", Handler: func() {}})

	first := mgr.SnapshotStart()
	mgr.Add(BucketStart, Sink{Owner: "later", Handler: func() {}})
	second := mgr.SnapshotStart()

	if len(first) != 1 {
		t.Fatalf("first snapshot len = %d, want 1", len(first))
	}
	if len(second) != 2 {
		t.Fatalf("second snapshot len = %d, want 2", len(second))
	}
}

func TestManagerSnapshotIsStableAcrossDeleteOwner(t *testing.T) {
	var mgr Manager
	mgr.Add(BucketClick, Sink{Owner: "keep", Handler: func() {}})
	mgr.Add(BucketClick, Sink{Owner: "drop", Handler: func() {}})

	first := mgr.SnapshotClick()
	mgr.DeleteOwner("drop")
	second := mgr.SnapshotClick()

	if len(first) != 2 {
		t.Fatalf("first snapshot len = %d, want 2", len(first))
	}
	if len(second) != 1 {
		t.Fatalf("second snapshot len = %d, want 1", len(second))
	}
	if second[0].Owner != "keep" {
		t.Fatalf("second snapshot owner = %v, want keep", second[0].Owner)
	}
}

func TestManagerSnapshotAppendDoesNotMutateBucket(t *testing.T) {
	var mgr Manager
	mgr.Add(BucketClick, Sink{Owner: "keep", Handler: func() {}})

	snapshot := mgr.SnapshotClick()
	snapshot = append(snapshot, Sink{Owner: "extra", Handler: func() {}})

	if len(snapshot) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snapshot))
	}
	if got := mgr.SnapshotClick(); len(got) != 1 {
		t.Fatalf("bucket len = %d, want 1", len(got))
	}
	if got := mgr.SnapshotClick()[0].Owner; got != "keep" {
		t.Fatalf("bucket owner = %v, want keep", got)
	}
}

func TestManagerConvenienceMethods(t *testing.T) {
	var mgr Manager
	mgr.AddClick(Sink{Owner: "click", Handler: func() {}})
	mgr.AddTimer(Sink{Owner: "timer", Handler: func() {}})

	if got := mgr.SnapshotClick(); len(got) != 1 || got[0].Owner != "click" {
		t.Fatalf("SnapshotClick = %+v, want click sink", got)
	}
	if got := mgr.SnapshotTimer(); len(got) != 1 || got[0].Owner != "timer" {
		t.Fatalf("SnapshotTimer = %+v, want timer sink", got)
	}
}

func TestHasOwner(t *testing.T) {
	sinks := []Sink{
		{Owner: "keep", Handler: func() {}},
		{Owner: "click", Handler: func() {}},
	}

	if !HasOwner(sinks, "click") {
		t.Fatal("HasOwner should find matching owner")
	}
	if HasOwner(sinks, "missing") {
		t.Fatal("HasOwner should return false for missing owner")
	}
}

func TestManagerStartLifecycle(t *testing.T) {
	var mgr Manager
	if !mgr.TryAddStart(Sink{Owner: "first", Handler: func() {}}) {
		t.Fatal("expected first start sink to be registered")
	}

	first := mgr.SnapshotStartOnce()
	if len(first) != 1 || first[0].Owner != "first" {
		t.Fatalf("SnapshotStartOnce = %+v, want first sink", first)
	}

	if mgr.TryAddStart(Sink{Owner: "late", Handler: func() {}}) {
		t.Fatal("expected late start sink to be rejected after first snapshot")
	}

	second := mgr.SnapshotStartOnce()
	if len(second) != 0 {
		t.Fatalf("second SnapshotStartOnce len = %d, want 0", len(second))
	}

	mgr.Reset()
	if !mgr.TryAddStart(Sink{Owner: "after-reset", Handler: func() {}}) {
		t.Fatal("expected reset to reopen start registration")
	}
}
