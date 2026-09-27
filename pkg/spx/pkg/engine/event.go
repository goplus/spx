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

package engine

import (
	"sort"
	"sync"
)

type Action0 func()

type Event0 struct {
	actions map[int]Action0
	nextID  int
	mutex   sync.Mutex
}

func NewEvent0() *Event0 {
	return &Event0{
		actions: make(map[int]Action0),
	}
}

func (e *Event0) Subscribe(action Action0) int {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	id := e.nextID
	e.actions[id] = action
	e.nextID++

	return id
}

func (e *Event0) Unsubscribe(id int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.actions, id)
}

func (e *Event0) UnsubscribeAll() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.actions = make(map[int]Action0)
}

func (e *Event0) Trigger() {
	e.mutex.Lock()
	actions := snapshotEventActions(e.actions)
	e.mutex.Unlock()
	for _, action := range actions {
		action()
	}
}

type Action1[T any] func(data T)

type Event1[T any] struct {
	actions map[int]Action1[T]
	nextID  int
	mutex   sync.Mutex
}

func NewEvent1[T any]() *Event1[T] {
	return &Event1[T]{
		actions: make(map[int]Action1[T]),
	}
}

func (e *Event1[T]) Subscribe(action Action1[T]) int {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	id := e.nextID
	e.actions[id] = action
	e.nextID++

	return id
}

func (e *Event1[T]) Unsubscribe(id int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.actions, id)
}
func (e *Event1[T]) UnsubscribeAll() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.actions = make(map[int]Action1[T])
}

func (e *Event1[T]) Trigger(data T) {
	e.mutex.Lock()
	actions := snapshotEventActions(e.actions)
	e.mutex.Unlock()
	for _, action := range actions {
		action(data)
	}
}

type Action2[T1 any, T2 any] func(data1 T1, data2 T2)

type Event2[T1 any, T2 any] struct {
	actions map[int]Action2[T1, T2]
	nextID  int
	mutex   sync.Mutex
}

func NewEvent2[T1 any, T2 any]() *Event2[T1, T2] {
	return &Event2[T1, T2]{
		actions: make(map[int]Action2[T1, T2]),
	}
}

func (e *Event2[T1, T2]) Subscribe(action Action2[T1, T2]) int {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	id := e.nextID
	e.actions[id] = action
	e.nextID++

	return id
}

func (e *Event2[T1, T2]) Unsubscribe(id int) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.actions, id)
}

func (e *Event2[T1, T2]) UnsubscribeAll() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.actions = make(map[int]Action2[T1, T2])
}

func (e *Event2[T1, T2]) Trigger(data1 T1, data2 T2) {
	e.mutex.Lock()
	actions := snapshotEventActions(e.actions)
	e.mutex.Unlock()
	for _, action := range actions {
		action(data1, data2)
	}
}

// snapshotEventActions is called under the event's mutex. Each dispatch owns its
// callbacks, so nested or concurrent triggers cannot change an active snapshot.
func snapshotEventActions[T any](registered map[int]T) []T {
	ids := make([]int, 0, len(registered))
	for id := range registered {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	actions := make([]T, 0, len(ids))
	for _, id := range ids {
		actions = append(actions, registered[id])
	}
	return actions
}
