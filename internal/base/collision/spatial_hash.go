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

type Entry[T any] struct {
	Value T
	Box   AABB
	seen  uint64
}

type SpatialHash[T any] struct {
	cellSize float64
	grid     map[int64]map[int64][]*Entry[T]
	queryID  uint64
}

func NewSpatialHash[T any](cellSize float64) *SpatialHash[T] {
	if cellSize <= 0 {
		panic("collision: cellSize must be greater than zero")
	}
	return &SpatialHash[T]{
		cellSize: cellSize,
		grid:     make(map[int64]map[int64][]*Entry[T]),
	}
}

func (s *SpatialHash[T]) Clear() {
	s.grid = make(map[int64]map[int64][]*Entry[T])
}

func (s *SpatialHash[T]) Insert(entry *Entry[T]) {
	if entry == nil {
		return
	}

	minCellX, minCellY := s.cellCoords(entry.Box.MinX, entry.Box.MinY)
	maxCellX, maxCellY := s.cellCoords(entry.Box.MaxX, entry.Box.MaxY)

	for x := minCellX; x <= maxCellX; x++ {
		if s.grid[x] == nil {
			s.grid[x] = make(map[int64][]*Entry[T])
		}
		for y := minCellY; y <= maxCellY; y++ {
			s.grid[x][y] = append(s.grid[x][y], entry)
		}
	}
}

func (s *SpatialHash[T]) Query(box AABB) []*Entry[T] {
	minCellX, minCellY := s.cellCoords(box.MinX, box.MinY)
	maxCellX, maxCellY := s.cellCoords(box.MaxX, box.MaxY)

	s.queryID++
	if s.queryID == 0 {
		s.queryID++
	}

	cellsWide := int(maxCellX-minCellX) + 1
	cellsHigh := int(maxCellY-minCellY) + 1
	if cellsWide < 0 {
		cellsWide = 0
	}
	if cellsHigh < 0 {
		cellsHigh = 0
	}
	results := make([]*Entry[T], 0, cellsWide*cellsHigh)

	for x := minCellX; x <= maxCellX; x++ {
		if s.grid[x] == nil {
			continue
		}
		for y := minCellY; y <= maxCellY; y++ {
			for _, candidate := range s.grid[x][y] {
				if candidate.seen == s.queryID {
					continue
				}
				candidate.seen = s.queryID
				results = append(results, candidate)
			}
		}
	}

	return results
}

func (s *SpatialHash[T]) cellCoords(x, y float64) (int64, int64) {
	return int64(x / s.cellSize), int64(y / s.cellSize)
}
