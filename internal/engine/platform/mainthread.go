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

package platform

import (
	"sync"
	"sync/atomic"

	"github.com/visualfc/gid"
)

var mainThreadDepth sync.Map // map[int64]*atomic.Int32
var mainThreadDepthPool = sync.Pool{
	New: func() any {
		return &atomic.Int32{}
	},
}

func currentMainThreadDepth() *atomic.Int32 {
	gid := gid.Get()
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
	gid := gid.Get()
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
	depth, ok := mainThreadDepth.Load(gid.Get())
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
