//go:build !pure_engine

package engine

import (
	"testing"

	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type positionQuerySpy struct {
	gdx.ISpriteMgr
	query func([]int64, []float32)
}

func (s *positionQuerySpy) BatchRetrievePositions(ids []int64, out []float32) {
	s.query(ids, out)
}

func TestPositionSyncReusesStorageAndPreservesIDs(t *testing.T) {
	previous := gdx.SpriteMgr
	enginewrap.Init(func(call func()) { call() })
	t.Cleanup(func() {
		gdx.SpriteMgr = previous
		enginewrap.Init(WaitMainThread)
	})
	ids := []int64{0x112233447fc00001, -1}
	calls := 0
	gdx.SpriteMgr = &positionQuerySpy{query: func(input []int64, out []float32) {
		calls++
		if len(input) != len(ids) || &input[0] != &ids[0] || input[0] != 0x112233447fc00001 || input[1] != -1 {
			t.Fatal("position query changed the original ID buffer")
		}
		if len(out) != len(ids)*2 {
			t.Fatal("position query has incorrect output length")
		}
		copy(out, []float32{float32(calls), -4, 5, -6})
	}}

	var buffer SpriteSyncBuffer
	first := buffer.GetPositions(ids)
	storage := &first[0]
	for range 2 {
		buffer.Clear()
		out := buffer.GetPositions(ids)
		if &out[0] != storage || out[0] != float32(calls) || out[3] != -6 {
			t.Fatal("position query did not refresh results in reusable storage")
		}
	}
	if out := buffer.GetPositions(nil); len(out) != 0 || calls != 3 {
		t.Fatal("empty position query reached the engine")
	}
	if out := buffer.GetPositions(ids); &out[0] != storage {
		t.Fatal("empty query discarded reusable storage")
	}

	var other SpriteSyncBuffer
	if out := other.GetPositions(ids); &out[0] == storage || first[0] != 4 {
		t.Fatal("independent sync buffers share position storage")
	}
	ids = append(ids, 2, 3, 4, 5, 6, 7, 8)
	if out := buffer.GetPositions(ids); len(out) != len(ids)*2 || out[0] != float32(calls) {
		t.Fatal("position storage did not grow with the batch")
	}
}
