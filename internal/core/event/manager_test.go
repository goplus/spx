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

import (
	"reflect"
	"runtime"
	"slices"
	"sync"
	"testing"
)

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

var registrationMethods = []struct {
	name   string
	bucket Bucket
	add    func(*Manager, Sink) bool
}{
	{"Add", BucketClick, func(mgr *Manager, sink Sink) bool {
		mgr.Add(BucketClick, sink)
		return true
	}},
	{"AddStart", BucketStart, func(mgr *Manager, sink Sink) bool {
		mgr.Add(BucketStart, sink)
		return true
	}},
	{"TryAddStart", BucketStart, (*Manager).TryAddStart},
}

func TestManagerSnapshotsAcrossRepeatedAppend(t *testing.T) {
	for _, method := range registrationMethods {
		for _, initial := range []struct {
			name     string
			capacity int
		}{
			{"zero value", 0},
			{"spare capacity", 16},
		} {
			t.Run(method.name+"/"+initial.name, func(t *testing.T) {
				var mgr Manager
				if initial.capacity > 0 {
					mgr.buckets[method.bucket] = make([]Sink, 0, initial.capacity)
				}
				var snapshots [][]Sink
				for i := range 12 {
					snapshot := mgr.Snapshot(method.bucket)
					if len(snapshot) != cap(snapshot) {
						t.Fatalf("snapshot len = %d, cap = %d", len(snapshot), cap(snapshot))
					}
					snapshots = append(snapshots, snapshot)
					// A local append must neither change the bucket nor be
					// overwritten by the next registration.
					local := append(snapshot, Sink{Owner: "local"})
					if got := mgr.Snapshot(method.bucket); !reflect.DeepEqual(got, snapshot) {
						t.Fatalf("local append changed bucket: %+v, want %+v", got, snapshot)
					}
					if !method.add(&mgr, Sink{Owner: i, Handler: i}) {
						t.Fatal("registration unexpectedly rejected")
					}
					if local[i].Owner != "local" {
						t.Fatalf("registration overwrote snapshot append: %+v", local[i])
					}
				}
				snapshots = append(snapshots, mgr.Snapshot(method.bucket))
				for n, snapshot := range snapshots {
					if len(snapshot) != n {
						t.Fatalf("snapshot %d len = %d, want %d", n, len(snapshot), n)
					}
					for i, sink := range snapshot {
						if sink.Owner != i || sink.Handler != i {
							t.Fatalf("snapshot %d element %d changed: %+v", n, i, sink)
						}
					}
				}
			})
		}
	}
}

func TestManagerSnapshotsSurviveDeleteResetAndReregistration(t *testing.T) {
	for _, method := range registrationMethods {
		t.Run(method.name, func(t *testing.T) {
			var mgr Manager
			original := []Sink{
				{Owner: "keep", Handler: 1},
				{Owner: "drop", Handler: 2},
				{Owner: "drop", Handler: 3},
				{Owner: "keep", Handler: 4},
				{Owner: "drop", Handler: 5},
			}
			for _, sink := range original {
				method.add(&mgr, sink)
			}
			beforeDelete := mgr.Snapshot(method.bucket)
			mgr.DeleteOwner("drop")
			afterDelete := mgr.Snapshot(method.bucket)
			kept := []Sink{original[0], original[3]}
			if !reflect.DeepEqual(afterDelete, kept) {
				t.Fatalf("after DeleteOwner = %+v, want %+v", afterDelete, kept)
			}
			for range 8 {
				method.add(&mgr, Sink{Owner: "later"})
			}
			beforeReset := mgr.Snapshot(method.bucket)
			wantBeforeReset := append([]Sink(nil), beforeReset...)
			mgr.Reset()
			for range 8 {
				if !method.add(&mgr, Sink{Owner: "after-reset"}) {
					t.Fatal("registration after Reset unexpectedly rejected")
				}
			}
			for _, check := range []struct {
				name string
				got  []Sink
				want []Sink
			}{
				{"before delete", beforeDelete, original},
				{"after delete", afterDelete, kept},
				{"before reset", beforeReset, wantBeforeReset},
			} {
				if !reflect.DeepEqual(check.got, check.want) {
					t.Errorf("%s snapshot = %+v, want %+v", check.name, check.got, check.want)
				}
			}
		})
	}
}

func TestManagerStartSnapshotSurvivesReset(t *testing.T) {
	var mgr Manager
	mgr.buckets[BucketStart] = make([]Sink, 0, 8)
	mgr.TryAddStart(Sink{Owner: "first"})
	snapshot := mgr.SnapshotStartOnce()
	if len(snapshot) != cap(snapshot) {
		t.Fatalf("start snapshot len = %d, cap = %d", len(snapshot), cap(snapshot))
	}
	local := append(snapshot, Sink{Owner: "local"})
	if mgr.TryAddStart(Sink{Owner: "late"}) {
		t.Fatal("registration accepted after start snapshot")
	}
	mgr.Reset()
	mgr.TryAddStart(Sink{Owner: "new"})
	if got := mgr.SnapshotStartOnce(); len(got) != 1 || got[0].Owner != "new" {
		t.Fatalf("new start snapshot = %+v", got)
	}
	if snapshot[0].Owner != "first" || local[1].Owner != "local" {
		t.Fatalf("old start snapshot changed: %+v, local append: %+v", snapshot, local)
	}
}

func TestManagerConcurrentSnapshotAndRegistration(t *testing.T) {
	for _, method := range registrationMethods {
		t.Run(method.name, func(t *testing.T) {
			var mgr Manager
			mgr.buckets[method.bucket] = make([]Sink, 0, 1024)
			method.add(&mgr, Sink{Owner: 0, Handler: 0})
			start := make(chan struct{})
			done := make(chan struct{})
			var readers sync.WaitGroup
			for range 4 {
				readers.Go(func() {
					old := mgr.Snapshot(method.bucket)
					<-start
					for {
						for _, snapshot := range [][]Sink{old, mgr.Snapshot(method.bucket)} {
							if len(snapshot) != cap(snapshot) {
								t.Errorf("snapshot len = %d, cap = %d", len(snapshot), cap(snapshot))
								return
							}
							for i, sink := range snapshot {
								if sink.Owner != i || sink.Handler != i {
									t.Errorf("snapshot element %d changed: %+v", i, sink)
									return
								}
							}
						}
						select {
						case <-done:
							return
						default:
						}
					}
				})
			}
			close(start)
			for i := 1; i < 1024; i++ {
				if !method.add(&mgr, Sink{Owner: i, Handler: i}) {
					t.Error("registration unexpectedly rejected")
					break
				}
				if i%8 == 0 {
					runtime.Gosched()
				}
			}
			close(done)
			readers.Wait()
		})
	}
}

func TestNewSink(t *testing.T) {
	handler := "handler"
	sink := NewSink("owner", handler)
	if sink.Owner != "owner" || sink.Handler != handler || sink.Cond != nil {
		t.Fatalf("NewSink = %+v", sink)
	}
}

func TestSinkMatchers(t *testing.T) {
	if !MatchOwner("sprite")("sprite") || MatchOwner("sprite")("other") {
		t.Fatal("MatchOwner mismatch")
	}
	if !MatchValue("msg")("msg") || MatchValue("msg")("other") {
		t.Fatal("MatchValue mismatch")
	}
	if !MatchAnyOf([]int{1, 2, 3})(2) || MatchAnyOf([]int{1, 2, 3})(4) {
		t.Fatal("MatchAnyOf mismatch")
	}
}

func TestMatchRisingEdge(t *testing.T) {
	for _, tt := range []struct {
		name   string
		values []bool
		want   []int
	}{
		{name: "initially false", values: []bool{false, false, true, true, false, true}, want: []int{2, 5}},
		{name: "initially true", values: []bool{true, true}, want: []int{0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			index := 0
			edge := MatchRisingEdge(func() bool {
				value := tt.values[index]
				index++
				return value
			})
			var fired []int
			for i := range tt.values {
				if edge(nil) {
					fired = append(fired, i)
				}
			}
			if !slices.Equal(fired, tt.want) {
				t.Fatalf("fired = %v, want %v", fired, tt.want)
			}
			if index != len(tt.values) {
				t.Fatalf("condition evaluated %d times, want %d", index, len(tt.values))
			}
		})
	}
}
