package platform

import (
	"sync"
	"sync/atomic"

	"github.com/petermattis/goid"
)

var mainThreadDepth sync.Map // map[int64]*atomic.Int32
var mainThreadDepthPool = sync.Pool{
	New: func() any {
		return &atomic.Int32{}
	},
}

func currentMainThreadDepth() *atomic.Int32 {
	gid := goid.Get()
	if depth, ok := mainThreadDepth.Load(gid); ok {
		return depth.(*atomic.Int32)
	}

	depth := mainThreadDepthPool.Get().(*atomic.Int32)
	depth.Store(0)
	actual, loaded := mainThreadDepth.LoadOrStore(gid, depth)
	if loaded {
		mainThreadDepthPool.Put(depth)
	}
	return actual.(*atomic.Int32)
}

func EnterMainThread() {
	currentMainThreadDepth().Add(1)
}

func ExitMainThread() {
	gid := goid.Get()
	depth, ok := mainThreadDepth.Load(gid)
	if !ok {
		return
	}

	counter := depth.(*atomic.Int32)
	if counter.Add(-1) <= 0 {
		mainThreadDepth.Delete(gid)
		counter.Store(0)
		mainThreadDepthPool.Put(counter)
	}
}

func IsMainThread() bool {
	depth, ok := mainThreadDepth.Load(goid.Get())
	if !ok {
		return false
	}
	return depth.(*atomic.Int32).Load() > 0
}

func RunOnMainThread(call func()) {
	EnterMainThread()
	defer ExitMainThread()
	call()
}
