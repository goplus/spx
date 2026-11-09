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
	registry eventRegistry[Action0]
}

type Action1[T any] func(data T)

type Event1[T any] struct {
	registry eventRegistry[Action1[T]]
}

type Action2[T1 any, T2 any] func(data1 T1, data2 T2)

type Event2[T1 any, T2 any] struct {
	registry eventRegistry[Action2[T1, T2]]
}

func NewEvent0() *Event0 {
	return &Event0{registry: eventRegistry[Action0]{actions: make(map[int]Action0)}}
}

func NewEvent1[T any]() *Event1[T] {
	return &Event1[T]{registry: eventRegistry[Action1[T]]{actions: make(map[int]Action1[T])}}
}

func NewEvent2[T1 any, T2 any]() *Event2[T1, T2] {
	return &Event2[T1, T2]{registry: eventRegistry[Action2[T1, T2]]{actions: make(map[int]Action2[T1, T2])}}
}

// Keep explicit public methods and named callback types for reflected bindings.
func (e *Event0) Subscribe(action Action0) int {
	return e.registry.subscribe(action)
}

func (e *Event0) Unsubscribe(id int) {
	e.registry.unsubscribe(id)
}

func (e *Event0) UnsubscribeAll() {
	e.registry.unsubscribeAll()
}

func (e *Event0) Trigger() {
	for _, action := range e.registry.snapshot() {
		action()
	}
}

func (e *Event1[T]) Subscribe(action Action1[T]) int {
	return e.registry.subscribe(action)
}

func (e *Event1[T]) Unsubscribe(id int) {
	e.registry.unsubscribe(id)
}

func (e *Event1[T]) UnsubscribeAll() {
	e.registry.unsubscribeAll()
}

func (e *Event1[T]) Trigger(data T) {
	for _, action := range e.registry.snapshot() {
		action(data)
	}
}

func (e *Event2[T1, T2]) Subscribe(action Action2[T1, T2]) int {
	return e.registry.subscribe(action)
}

func (e *Event2[T1, T2]) Unsubscribe(id int) {
	e.registry.unsubscribe(id)
}

func (e *Event2[T1, T2]) UnsubscribeAll() {
	e.registry.unsubscribeAll()
}

func (e *Event2[T1, T2]) Trigger(data1 T1, data2 T2) {
	for _, action := range e.registry.snapshot() {
		action(data1, data2)
	}
}

type eventRegistry[A any] struct {
	actions map[int]A
	nextID  int
	mutex   sync.Mutex
}

func (r *eventRegistry[A]) subscribe(action A) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	id := r.nextID
	r.actions[id] = action
	r.nextID++
	return id
}

func (r *eventRegistry[A]) unsubscribe(id int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.actions, id)
}

func (r *eventRegistry[A]) unsubscribeAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.actions = make(map[int]A)
}

// Each dispatch owns a sorted snapshot and invokes callbacks after unlocking,
// so subscription changes and nested or concurrent triggers are independent.
func (r *eventRegistry[A]) snapshot() []A {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	ids := make([]int, 0, len(r.actions))
	for id := range r.actions {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	actions := make([]A, 0, len(ids))
	for _, id := range ids {
		actions = append(actions, r.actions[id])
	}
	return actions
}
