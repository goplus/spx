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

package collision

import "testing"

func TestAABBIntersects(t *testing.T) {
	a := AABB{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}
	b := AABB{MinX: 5, MinY: 5, MaxX: 15, MaxY: 15}
	c := AABB{MinX: 11, MinY: 11, MaxX: 20, MaxY: 20}

	if !a.Intersects(b) {
		t.Fatal("expected overlapping boxes to intersect")
	}
	if a.Intersects(c) {
		t.Fatal("expected separated boxes not to intersect")
	}
}
