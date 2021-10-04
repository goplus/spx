/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

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
