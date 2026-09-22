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

package project

import (
	"fmt"
	"strconv"

	"github.com/goplus/spx/v3/internal/tools"
)

// CostumeFrame identifies one expanded costume frame. Part is zero for a
// CostumeSet and the source part index for a CostumeMPSet.
type CostumeFrame struct {
	Name  string
	Part  int
	Index int
}

// ResolveFrameIndex applies the animation frame-reference rules to this
// layout. Strings name costumes; supported numeric values are truncated to an
// index. Other values retain the historical fallback to frame zero.
func (l *CostumeLayout) ResolveFrameIndex(value any) (int, bool) {
	if name, ok := value.(string); ok {
		index, exists := l.nameIndexes[name]
		return index, exists
	}
	valueAsFloat, ok := tools.GetFloat(value)
	if !ok {
		return 0, true
	}
	return int(valueAsFloat), true
}

// CostumeLayout is the validated, expanded view of a sprite's costumes.
type CostumeLayout struct {
	Frames      []CostumeFrame
	nameIndexes map[string]int
}

// PrepareCostumeLayout validates and expands the costume declaration selected
// by SpriteConfig's precedence rules.
func PrepareCostumeLayout(cfg *SpriteConfig) (*CostumeLayout, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	layout := &CostumeLayout{nameIndexes: make(map[string]int)}
	switch {
	case cfg.Costumes != nil:
		if len(cfg.Costumes) == 0 {
			return nil, fmt.Errorf("costumes must not be empty")
		}
		layout.Frames = make([]CostumeFrame, 0, len(cfg.Costumes))
		for i, costume := range cfg.Costumes {
			if costume == nil {
				return nil, fmt.Errorf("costumes[%d] is null", i)
			}
			layout.appendFrame(CostumeFrame{Name: costume.Name, Index: i})
		}

	case cfg.CostumeSet != nil:
		if err := layout.appendSet(0, cfg.CostumeSet.Nx, cfg.CostumeSet.Items); err != nil {
			return nil, fmt.Errorf("costumeSet: %w", err)
		}

	case cfg.CostumeMPSet != nil:
		if len(cfg.CostumeMPSet.Parts) == 0 {
			return nil, fmt.Errorf("costumeMPSet.parts must not be empty")
		}
		for i, part := range cfg.CostumeMPSet.Parts {
			if err := layout.appendSet(i, part.Nx, part.Items); err != nil {
				return nil, fmt.Errorf("costumeMPSet.parts[%d]: %w", i, err)
			}
		}

	default:
		return nil, fmt.Errorf("configuration must define costumes, costumeSet, or costumeMPSet")
	}
	return layout, nil
}

func (l *CostumeLayout) appendSet(part, count int, items []CostumeSetItem) error {
	if count <= 0 {
		return fmt.Errorf("invalid frame count %d", count)
	}

	start := len(l.Frames)
	if count == 1 || items == nil {
		for i := 0; i < count; i++ {
			l.appendFrame(CostumeFrame{
				Name:  strconv.Itoa(start + i),
				Part:  part,
				Index: i,
			})
		}
		return nil
	}

	frameIndex := 0
	for itemIndex, item := range items {
		if item.N < 0 {
			return fmt.Errorf("items[%d] has negative frame count %d", itemIndex, item.N)
		}
		for i := 0; i < item.N; i++ {
			l.appendFrame(CostumeFrame{
				Name:  item.NamePrefix + strconv.Itoa(i),
				Part:  part,
				Index: frameIndex,
			})
			frameIndex++
		}
	}
	if frameIndex != count {
		return fmt.Errorf("incomplete frame loading (loaded=%d, expected=%d)", frameIndex, count)
	}
	return nil
}

func (l *CostumeLayout) appendFrame(frame CostumeFrame) {
	index := len(l.Frames)
	l.Frames = append(l.Frames, frame)
	if _, exists := l.nameIndexes[frame.Name]; !exists {
		l.nameIndexes[frame.Name] = index
	}
}
