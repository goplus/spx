package engine

import "testing"

func TestEvent0TriggerClearsTempActions(t *testing.T) {
	ev := NewEvent0()
	ev.Subscribe(func() {})
	ev.Subscribe(func() {})

	ev.Trigger()

	for i, action := range ev.tempActions {
		if action != nil {
			t.Fatalf("tempActions[%d] should be cleared after trigger", i)
		}
	}
	if len(ev.tempIds) != 0 {
		t.Fatalf("tempIds should be cleared after trigger, got %d", len(ev.tempIds))
	}
}

func TestEvent2TriggerClearsShrunkTempActions(t *testing.T) {
	ev := NewEvent2[int, int]()
	firstID := ev.Subscribe(func(int, int) {})
	ev.Subscribe(func(int, int) {})

	ev.Trigger(1, 2)
	ev.Unsubscribe(firstID)
	ev.Trigger(3, 4)

	for i, action := range ev.tempActions {
		if action != nil {
			t.Fatalf("tempActions[%d] should be cleared after trigger", i)
		}
	}
	if len(ev.tempIds) != 0 {
		t.Fatalf("tempIds should be cleared after trigger, got %d", len(ev.tempIds))
	}
}
