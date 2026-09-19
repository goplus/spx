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

package facade

import "sync"

// LinkSession keeps startup and cleanup tied to one backend link.
type LinkSession struct {
	run     func(func())
	unlink  func()
	mu      sync.Mutex
	started bool
	closed  bool
}

func (s *LinkSession) Run(ready func()) {
	s.mu.Lock()
	if s.closed || s.started {
		s.mu.Unlock()
		if ready != nil {
			ready()
		}
		return
	}
	s.started = true
	releaseStartup := sync.OnceFunc(s.mu.Unlock)
	defer releaseStartup()
	s.run(func() {
		releaseStartup()
		if ready != nil {
			ready()
		}
	})
}

func (s *LinkSession) Unlink() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.unlink != nil {
		s.unlink()
	}
}

func newLinkSession(run func(func()), unlink func()) *LinkSession {
	return &LinkSession{run: run, unlink: unlink}
}
