package coroutine

import (
	"testing"
	"time"
)

func BenchmarkDrainExternalTask(b *testing.B) {
	co := New(nil)
	b.ReportAllocs()
	for b.Loop() {
		if !co.admitWorker(nil) {
			b.Fatal("external task was rejected")
		}
		// Include a short external operation; timer precision depends on the host.
		time.AfterFunc(100*time.Microsecond, co.finishWorker)
		if !co.waitForDrain(time.Second, nil) {
			b.Fatal("external task did not drain")
		}
	}
}
