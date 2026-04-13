package spx

import (
	"reflect"
	"testing"
)

func TestGameRunAfterAwakeExecutesQueuedHooksOnce(t *testing.T) {
	var g Game

	var got []string
	g.deferAfterAwake(func() {
		got = append(got, "first")
	})
	g.deferAfterAwake(func() {
		got = append(got, "second")
	})

	g.runAfterAwake()
	g.runAfterAwake()

	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runAfterAwake got %v, want %v", got, want)
	}
}
