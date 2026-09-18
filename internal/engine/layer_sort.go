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

package engine

import "fmt"

type layerSortMode int

const (
	layerSortModeNone layerSortMode = iota
	layerSortModeVertical
)

type LayerSortInfo struct {
	X      float64
	Y      float64
	Sprite *Sprite
}

var currentLayerSortMode layerSortMode

// SetLayerSortMode selects sprite sorting by name.
func SetLayerSortMode(name string) error {
	mode := layerSortModeNone
	switch name {
	case "", "none":
	case "vertical":
		mode = layerSortModeVertical
	default:
		return fmt.Errorf("unknown layer sort mode: %s", name)
	}

	currentLayerSortMode = mode
	Managers().ExtMgr.SetLayerSorterMode(int64(mode))
	return nil
}

func HasLayerSortMethod() bool {
	return currentLayerSortMode != layerSortModeNone
}
