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

type AABB struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

func (a AABB) Intersects(b AABB) bool {
	return a.MinX <= b.MaxX &&
		a.MaxX >= b.MinX &&
		a.MinY <= b.MaxY &&
		a.MaxY >= b.MinY
}
