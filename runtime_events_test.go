package spx

import (
	"reflect"
	"testing"
)

func TestGameRunBootstrapTasksExecutesQueuedHooksOnce(t *testing.T) {
	var g Game

	var got []string
	g.deferBootstrap(func() {
		got = append(got, "first")
	})
	g.deferBootstrap(func() {
		got = append(got, "second")
	})

	g.runBootstrapTasks()
	g.runBootstrapTasks()

	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runBootstrapTasks got %v, want %v", got, want)
	}
}
