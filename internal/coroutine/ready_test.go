package coroutine

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// The reference scheduler stores explicit round numbers and sorts a flat list.
// It does not share heap membership or removal logic with readyQueue.
type readyModel struct {
	round  int
	cursor int64
	items  []readyModelItem
}

type readyModelItem struct {
	thread Thread
	round  int
}

func (m *readyModel) add(th Thread) {
	round := m.round
	if th.resumeOrder <= m.cursor {
		round++
	}
	m.items = append(m.items, readyModelItem{th, round})
}

func (m *readyModel) pop() Thread {
	slices.SortFunc(m.items, func(a, b readyModelItem) int {
		if c := cmp.Compare(a.round, b.round); c != 0 {
			return c
		}
		if c := cmp.Compare(a.thread.resumeOrder, b.thread.resumeOrder); c != 0 {
			return c
		}
		return cmp.Compare(a.thread.id, b.thread.id)
	})
	for len(m.items) != 0 {
		item := m.items[0]
		m.items = m.items[1:]
		m.round = item.round
		if !item.thread.canceled() {
			m.cursor = item.thread.resumeOrder
			return item.thread
		}
	}
	return nil
}

func TestReadyQueueMatchesRoundModel(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			scheduler := New(nil)
			random := rand.New(rand.NewPCG(seed, seed+1))
			var q readyQueue
			var model readyModel
			pop := func(step int) Thread {
				t.Helper()
				got, want := q.pop(), model.pop()
				if got != want {
					t.Fatalf("step %d: selected %v, want %v", step, got, want)
				}
				if got != nil && got.readyHeap != nil {
					t.Fatalf("step %d: selected thread remains queued", step)
				}
				return got
			}
			for step := range 1000 {
				switch random.IntN(6) {
				case 0, 1:
					th := scheduler.newThread(nil)
					// Include equal positions, as with restarted handlers.
					th.resumeOrder = int64(random.IntN(32) + 1)
					q.add(th)
					model.add(th)
				case 2:
					pop(step)
				case 3:
					if len(model.items) != 0 {
						model.items[random.IntN(len(model.items))].thread.Cancel()
					}
				case 4:
					if len(model.items) != 0 {
						i := random.IntN(len(model.items))
						model.items[i].thread.removeReadyLocked()
						model.items = slices.Delete(model.items, i, i+1)
					}
				case 5:
					if th := pop(step); th != nil {
						q.add(th)
						model.add(th)
					}
				}
				if count := len(q.heaps[0]) + len(q.heaps[1]); count != len(model.items) || q.cursor != model.cursor {
					t.Fatalf("step %d: queue count/cursor = %d/%d, want %d/%d", step, count, q.cursor, len(model.items), model.cursor)
				}
				for _, item := range model.items {
					th := item.thread
					if th.readyHeap == nil || (*th.readyHeap)[th.readyIndex] != th {
						t.Fatalf("step %d: invalid membership for %v", step, th)
					}
				}
			}
			for len(model.items) != 0 {
				pop(1000)
			}
			pop(1001)
		})
	}
}
