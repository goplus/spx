package spx

import (
	"testing"
	"time"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
)

func TestQueueBlockDoesNotBlockManagedCoroutine(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	game.events = make(chan event, 1)
	game.events <- &eventTimer{Time: 1}
	game.setEventQueuePolicy(coreevent.QueueBlock)

	done := make(chan bool, 1)
	thread := co.Create("managed-producer", func(coroutine.Thread) int {
		done <- game.queueEventWithPolicy(&eventTimer{Time: 2})
		return 0
	})

	select {
	case accepted := <-done:
		if accepted {
			t.Fatal("managed QueueBlock producer accepted an item into a full queue")
		}
	case <-time.After(time.Second):
		// Release a direct-send implementation so cleanup cannot strand the
		// scheduler lock after reporting the regression.
		<-game.events
		t.Fatal("managed QueueBlock producer blocked while holding the scheduler run slot")
	}
	co.Join(thread)

	if got := game.gameRuntimeState.EventQueueStats.DroppedTotal(); got != 1 {
		t.Fatalf("DroppedTotal = %d, want 1", got)
	}
	if got := <-game.events; got.(*eventTimer).Time != 1 {
		t.Fatal("managed fallback changed the existing queue item")
	}
}

func TestQueueBlockStillBlocksExternalCaller(t *testing.T) {
	_, game := setupRuntimeEventGame(t)
	game.events = make(chan event, 1)
	game.events <- &eventTimer{Time: 1}
	game.setEventQueuePolicy(coreevent.QueueBlock)

	started := make(chan struct{})
	done := make(chan bool, 1)
	go func() {
		close(started)
		done <- game.queueEventWithPolicy(&eventTimer{Time: 2})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("external QueueBlock caller did not start")
	}

	select {
	case <-done:
		t.Fatal("external QueueBlock caller returned before capacity was available")
	case <-time.After(50 * time.Millisecond):
	}
	<-game.events
	select {
	case accepted := <-done:
		if !accepted {
			t.Fatal("external QueueBlock caller did not enqueue after capacity was released")
		}
	case <-time.After(time.Second):
		t.Fatal("external QueueBlock caller did not resume after capacity was released")
	}
}
