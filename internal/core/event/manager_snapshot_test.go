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
	"sync"
	"testing"
)

var registrationMethods = []struct {
	name   string
	bucket Bucket
	add    func(*Manager, Sink) bool
}{
	{"Add", BucketClick, func(mgr *Manager, sink Sink) bool {
		mgr.Add(BucketClick, sink)
		return true
	}},
	{"TryAddStart", BucketStart, (*Manager).TryAddStart},
}

func TestManagerSnapshotsAcrossRepeatedAppend(t *testing.T) {
	for _, method := range registrationMethods {
		t.Run(method.name, func(t *testing.T) {
			var mgr Manager
			// Leave spare capacity to exercise readers sharing an array with
			// subsequent appends, including an initially empty snapshot.
			mgr.buckets[method.bucket] = make([]Sink, 0, 16)
			var snapshots [][]Sink
			for i := range 12 {
				snapshot := mgr.Snapshot(method.bucket)
				if len(snapshot) != cap(snapshot) {
					t.Fatalf("snapshot len = %d, cap = %d", len(snapshot), cap(snapshot))
				}
				snapshots = append(snapshots, snapshot)
				// Appending to a snapshot must not reserve or overwrite the
				// next registration, even when the bucket has spare capacity.
				local := append(snapshot, Sink{Owner: "local"})
				if !method.add(&mgr, Sink{Owner: i, Handler: i}) {
					t.Fatal("registration unexpectedly rejected")
				}
				if local[i].Owner != "local" {
					t.Fatalf("registration overwrote snapshot append: %+v", local[i])
				}
			}
			for n, snapshot := range snapshots {
				if len(snapshot) != n {
					t.Fatalf("snapshot %d len = %d", n, len(snapshot))
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
