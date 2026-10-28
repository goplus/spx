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

package ui

import "sort"

type sayBubbleScore struct {
	outsideArea  float64
	overlapArea  float64
	flippedCount int
	changedCount int
}

// ResolveSayBubbleLayouts chooses a direction for each bubble. Components of
// up to sayBubbleExactLayoutLimit bubbles are solved globally, avoiding local
// minima from one-at-a-time flipping. Larger components use deterministic
// multi-start local search. Both paths use stable IDs, not slice order.
func ResolveSayBubbleLayouts(layouts []SayBubbleLayout) {
	if len(layouts) == 0 {
		return
	}
	for i := range layouts {
		layouts[i].isLeft = layouts[i].preferredIsLeft
	}

	order := canonicalSayBubbleOrder(layouts)
	visited := make([]bool, len(layouts))
	queue := make([]int, 0, len(layouts))
	component := make([]int, 0, len(layouts))
	for _, seed := range order {
		if visited[seed] {
			continue
		}
		queue = append(queue[:0], seed)
		component = component[:0]
		visited[seed] = true
		for len(queue) > 0 {
			index := queue[0]
			queue = queue[1:]
			component = append(component, index)
			for _, other := range order {
				if visited[other] || !sayBubblesMayOverlap(layouts[index], layouts[other]) {
					continue
				}
				visited[other] = true
				queue = append(queue, other)
			}
		}

		sort.Slice(component, func(i, j int) bool {
			return sayBubbleCanonicalLess(layouts[component[i]], layouts[component[j]])
		})
		if len(component) <= sayBubbleExactLayoutLimit {
			resolveSayBubbleComponentExact(layouts, component)
		} else {
			resolveSayBubbleComponentFallback(layouts, component)
		}
	}
}

func (s sayBubbleScore) less(other sayBubbleScore) bool {
	if s.outsideArea != other.outsideArea {
		return s.outsideArea < other.outsideArea
	}
	if s.overlapArea != other.overlapArea {
		return s.overlapArea < other.overlapArea
	}
	if s.flippedCount != other.flippedCount {
		return s.flippedCount < other.flippedCount
	}
	return s.changedCount < other.changedCount
}

func (s sayBubbleScore) equal(other sayBubbleScore) bool {
	return s == other
}

// addCandidate scores the bubble itself and returns its collision bounds,
// which include the safety gap used only for overlap scoring.
func (s *sayBubbleScore) addCandidate(layout *SayBubbleLayout, isLeft bool) sayBubbleRect {
	rect := layout.candidate(isLeft)
	s.outsideArea += rect.outsideArea(layout.viewport)
	s.flippedCount += boolInt(isLeft != layout.preferredIsLeft)
	if layout.hasPrevious {
		s.changedCount += boolInt(isLeft != layout.previousIsLeft)
	}
	return rect.grow(sayBubbleGap / 2)
}

func canonicalSayBubbleOrder(layouts []SayBubbleLayout) []int {
	order := make([]int, len(layouts))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return sayBubbleCanonicalLess(layouts[order[i]], layouts[order[j]])
	})
	return order
}

func sayBubbleCanonicalLess(a, b SayBubbleLayout) bool {
	if a.stableID != b.stableID {
		return a.stableID < b.stableID
	}
	if a.position.X != b.position.X {
		return a.position.X < b.position.X
	}
	if a.position.Y != b.position.Y {
		return a.position.Y < b.position.Y
	}
	if a.extent.X != b.extent.X {
		return a.extent.X < b.extent.X
	}
	if a.extent.Y != b.extent.Y {
		return a.extent.Y < b.extent.Y
	}
	if a.preferredIsLeft != b.preferredIsLeft {
		return !a.preferredIsLeft
	}
	if a.content.style != b.content.style {
		return a.content.style < b.content.style
	}
	if a.content.formattedMessage != b.content.formattedMessage {
		return a.content.formattedMessage < b.content.formattedMessage
	}
	if a.hasPrevious != b.hasPrevious {
		return !a.hasPrevious
	}
	return !a.previousIsLeft && b.previousIsLeft
}

func sayBubblesMayOverlap(a, b SayBubbleLayout) bool {
	for _, aIsLeft := range [...]bool{false, true} {
		aRect := a.candidate(aIsLeft).grow(sayBubbleGap / 2)
		for _, bIsLeft := range [...]bool{false, true} {
			bRect := b.candidate(bIsLeft).grow(sayBubbleGap / 2)
			if aRect.overlapArea(bRect) > 0 {
				return true
			}
		}
	}
	return false
}

func resolveSayBubbleComponentExact(layouts []SayBubbleLayout, component []int) {
	combinationCount := uint64(1) << uint(len(component))
	var directionStorage [sayBubbleExactLayoutLimit]bool
	directions := directionStorage[:len(component)]
	var bestMask uint64
	var bestScore sayBubbleScore
	hasBest := false
	for mask := uint64(0); mask < combinationCount; mask++ {
		for i := range directions {
			directions[i] = mask&(uint64(1)<<uint(i)) != 0
		}
		score := scoreSayBubbleDirections(layouts, component, directions)
		if !hasBest || score.less(bestScore) ||
			(score.equal(bestScore) && sayBubbleMaskLess(mask, bestMask, len(component))) {
			bestMask = mask
			bestScore = score
			hasBest = true
		}
	}
	for i, index := range component {
		layouts[index].isLeft = bestMask&(uint64(1)<<uint(i)) != 0
	}
}

func sayBubbleMaskLess(a, b uint64, count int) bool {
	for i := 0; i < count; i++ {
		aIsLeft := a&(uint64(1)<<uint(i)) != 0
		bIsLeft := b&(uint64(1)<<uint(i)) != 0
		if aIsLeft != bIsLeft {
			return !aIsLeft
		}
	}
	return false
}

func resolveSayBubbleComponentFallback(layouts []SayBubbleLayout, component []int) {
	bestDirections := make([]bool, len(component))
	directions := make([]bool, len(component))
	var bestScore sayBubbleScore
	hasBest := false

	for seed := 0; seed < sayBubbleFallbackSeeds; seed++ {
		for i, index := range component {
			layout := layouts[index]
			switch seed {
			case 0:
				directions[i] = layout.preferredIsLeft
			case 1:
				if layout.hasPrevious {
					directions[i] = layout.previousIsLeft
				} else {
					directions[i] = layout.preferredIsLeft
				}
			case 2:
				directions[i] = false
			case 3:
				directions[i] = true
			default:
				directions[i] = !layout.preferredIsLeft
			}
		}

		for pass := 0; pass < len(component); pass++ {
			changed := false
			for i, currentIsLeft := range directions {
				current := scoreSayBubbleAt(layouts, component, directions, i, currentIsLeft)
				alternate := scoreSayBubbleAt(layouts, component, directions, i, !currentIsLeft)
				if alternate.less(current) {
					directions[i] = !currentIsLeft
					changed = true
				}
			}
			if !changed {
				break
			}
		}

		score := scoreSayBubbleDirections(layouts, component, directions)
		if !hasBest || score.less(bestScore) ||
			(score.equal(bestScore) && sayBubbleDirectionsLess(directions, bestDirections)) {
			copy(bestDirections, directions)
			bestScore = score
			hasBest = true
		}
	}

	for i, index := range component {
		layouts[index].isLeft = bestDirections[i]
	}
}

func scoreSayBubbleDirections(layouts []SayBubbleLayout, component []int, directions []bool) sayBubbleScore {
	score := sayBubbleScore{}
	for i, index := range component {
		rect := score.addCandidate(&layouts[index], directions[i])
		for j, otherIndex := range component[i+1:] {
			other := layouts[otherIndex].candidate(directions[i+j+1]).grow(sayBubbleGap / 2)
			score.overlapArea += rect.overlapArea(other)
		}
	}
	return score
}

func scoreSayBubbleAt(layouts []SayBubbleLayout, component []int, directions []bool, at int, isLeft bool) sayBubbleScore {
	score := sayBubbleScore{}
	candidate := score.addCandidate(&layouts[component[at]], isLeft)
	for i, otherIndex := range component {
		if i == at {
			continue
		}
		occupied := layouts[otherIndex].candidate(directions[i]).grow(sayBubbleGap / 2)
		score.overlapArea += candidate.overlapArea(occupied)
	}
	return score
}

func sayBubbleDirectionsLess(a, b []bool) bool {
	for i := range a {
		if a[i] != b[i] {
			return !a[i]
		}
	}
	return false
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
