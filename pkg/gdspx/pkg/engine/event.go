package engine

import (
	"sort"
	"sync"
)

type Action0 func()

type Event0 struct {
	actions     map[int]Action0
	nextID      int
	mutex       sync.Mutex
	tempActions []Action0
	tempIds     []int
}

func NewEvent0() *Event0 {
	return &Event0{
		actions:     make(map[int]Action0),
		nextID:      0,
		mutex:       sync.Mutex{},
		tempActions: make([]Action0, 0),
		tempIds:     make([]int, 0),
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
	for id := range e.actions {
		e.tempIds = append(e.tempIds, id)
	}
	sort.Ints(e.tempIds)
	count := len(e.tempIds)
	curCount := len(e.tempActions)
	for i := curCount; i < count; i++ {
		e.tempActions = append(e.tempActions, nil)
	}
	for i, id := range e.tempIds {
		e.tempActions[i] = e.actions[id]
	}
	e.mutex.Unlock()
	for i := 0; i < count; i++ {
		e.tempActions[i]()
	}
	e.tempIds = e.tempIds[:0]
}

type Action1[T any] func(data T)

type Event1[T any] struct {
	name        string
	actions     map[int]Action1[T]
	nextID      int
	mutex       sync.Mutex
	tempActions []Action1[T]
	tempIds     []int
}

func NewEvent1[T any]() *Event1[T] {
	return &Event1[T]{
		name:        "aaa",
		actions:     make(map[int]Action1[T]),
		nextID:      0,
		mutex:       sync.Mutex{},
		tempActions: make([]Action1[T], 0),
		tempIds:     make([]int, 0),
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
	for id := range e.actions {
		e.tempIds = append(e.tempIds, id)
	}
	sort.Ints(e.tempIds)
	count := len(e.tempIds)
	curCount := len(e.tempActions)
	for i := curCount; i < count; i++ {
		e.tempActions = append(e.tempActions, nil)
	}
	for i, id := range e.tempIds {
		e.tempActions[i] = e.actions[id]
	}
	e.mutex.Unlock()
	for i := 0; i < count; i++ {
		e.tempActions[i](data)
	}
	e.tempIds = e.tempIds[:0]
}

type Action2[T1 any, T2 any] func(data1 T1, data2 T2)

type Event2[T1 any, T2 any] struct {
	actions     map[int]Action2[T1, T2]
	nextID      int
	mutex       sync.Mutex
	tempActions []Action2[T1, T2]
	tempIds     []int
}

func NewEvent2[T1 any, T2 any]() *Event2[T1, T2] {
	return &Event2[T1, T2]{
		actions:     make(map[int]Action2[T1, T2]),
		nextID:      0,
		mutex:       sync.Mutex{},
		tempActions: make([]Action2[T1, T2], 0),
		tempIds:     make([]int, 0),
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
	for id := range e.actions {
		e.tempIds = append(e.tempIds, id)
	}
	sort.Ints(e.tempIds)
	count := len(e.tempIds)
	curCount := len(e.tempActions)
	for i := curCount; i < count; i++ {
		e.tempActions = append(e.tempActions, nil)
	}
	for i, id := range e.tempIds {
		e.tempActions[i] = e.actions[id]
	}
	e.mutex.Unlock()
	for i := 0; i < count; i++ {
		e.tempActions[i](data1, data2)
	}
	e.tempIds = e.tempIds[:0]
}
