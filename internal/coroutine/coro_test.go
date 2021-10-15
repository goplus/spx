package coroutine

import (
	"testing"
)

func TestCoroutine(t *testing.T) {
	co := New()
	resume := make(chan bool)

	var array []int
	co.Create(nil, func(th Thread) int {
		for i := 1; i <= 10; i++ {
			array = append(array, i+1)
			go func() {
				<-resume
				co.Resume(th)
			}()
			co.Yield(th)
		}
		return 0
	})

	for j := 1; j <= 5; j++ {
		resume <- true
	}

	if len(array) < 5 {
		t.Fatal("len(array):", len(array))
	}
}
