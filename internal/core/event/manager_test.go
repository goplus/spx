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
	for bucket := Bucket(0); bucket < bucketCount; bucket++ {
		mgr.Add(bucket, Sink{Owner: "sprite", Handler: func() {}})
	}

	mgr.Reset()

	for bucket := Bucket(0); bucket < bucketCount; bucket++ {
		if got := mgr.Snapshot(bucket); len(got) != 0 {
			t.Fatalf("bucket %d len = %d, want 0", bucket, len(got))
		}
	}
}

func TestManagerDeleteOwner(t *testing.T) {
	var mgr Manager
	mgr.Add(BucketStart, Sink{Owner: "keep", Handler: func() {}})
	mgr.Add(BucketClick, Sink{Owner: "drop", Handler: func() {}})
	mgr.Add(BucketTimer, Sink{Owner: "drop", Handler: func() {}})
	mgr.Add(BucketCondition, Sink{Owner: "drop", Handler: func() {}})

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
	if got := mgr.Snapshot(BucketCondition); len(got) != 0 {
		t.Fatalf("BucketCondition len = %d, want 0", len(got))
	}
}

func TestManagerBucketsAreIndependent(t *testing.T) {
	var mgr Manager
	for bucket := Bucket(0); bucket < bucketCount; bucket++ {
		mgr.Add(bucket, Sink{Owner: bucket, Handler: func() {}})
	}

	for bucket := Bucket(0); bucket < bucketCount; bucket++ {
		if got := mgr.Snapshot(bucket); len(got) != 1 || got[0].Owner != bucket {
			t.Fatalf("bucket %d = %+v, want its own sink", bucket, got)
		}
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
