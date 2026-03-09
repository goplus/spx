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

func TestManagerSnapshotStartOnce(t *testing.T) {
	var mgr Manager
	mgr.Add(BucketStart, Sink{Owner: "game", Handler: func() {}})

	first := mgr.SnapshotStartOnce()
	second := mgr.SnapshotStartOnce()

	if len(first) != 1 {
		t.Fatalf("first snapshot len = %d, want 1", len(first))
	}
	if len(second) != 0 {
		t.Fatalf("second snapshot len = %d, want 0", len(second))
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
