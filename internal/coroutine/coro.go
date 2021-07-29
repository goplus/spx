/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package coroutine

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	// ErrCannotYieldANonrunningThread represents an "can not yield a non-running thread" error.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")
)

// -------------------------------------------------------------------------------------

// Thread represents a coroutine id.
//
type Thread int

// Coroutines represents a coroutine manager.
//
type Coroutines struct {
	suspended map[Thread]bool
	nextID    int64
	current   int64
	mutex     sync.Mutex
	cond      sync.Cond
	sema      sync.Mutex
}

// New creates a coroutine manager.
//
func New() *Coroutines {
	p := &Coroutines{
		suspended: make(map[Thread]bool),
	}
	p.cond.L = &p.mutex
	return p
}

// Create creates a new coroutine.
//
func (p *Coroutines) Create(fn func(me Thread) int) Thread {
	return p.CreateAndStart(fn, 0)
}

// CreateAndStart creates and executes the new coroutine.
//
func (p *Coroutines) CreateAndStart(fn func(me Thread) int, main Thread) Thread {
	id := Thread(atomic.AddInt64(&p.nextID, 1))
	go func() {
		p.sema.Lock()
		atomic.StoreInt64(&p.current, int64(id))
		fn(id)
		p.mutex.Lock()
		delete(p.suspended, id)
		p.mutex.Unlock()
		p.notify(id)
		p.sema.Unlock()
	}()
	if main != 0 {
		runtime.Gosched()
	}
	return id
}

func (p *Coroutines) Current() Thread {
	return Thread(atomic.LoadInt64(&p.current))
}

func (p *Coroutines) notify(me Thread) {
}

// Yield suspends a running coroutine.
//
func (p *Coroutines) Yield(me Thread) {
	if atomic.LoadInt64(&p.current) != int64(me) {
		panic(ErrCannotYieldANonrunningThread)
	}
	p.notify(me)
	p.sema.Unlock()

	p.mutex.Lock()
	p.suspended[me] = true
	for p.suspended[me] {
		p.cond.Wait()
	}
	p.mutex.Unlock()

	p.sema.Lock()
	p.current = int64(me)
}

// Resume resumes a suspended coroutine.
//
func (p *Coroutines) Resume(th Thread) {
	for {
		done := false
		p.mutex.Lock()
		if p.suspended[th] {
			p.suspended[th] = false
			p.cond.Broadcast()
			done = true
		}
		p.mutex.Unlock()
		if done {
			return
		}
		runtime.Gosched()
	}
}

// Sched func.
//
func (p *Coroutines) Sched(me Thread) {
	go func() {
		p.Resume(me)
	}()
	p.Yield(me)
}

// -------------------------------------------------------------------------------------
