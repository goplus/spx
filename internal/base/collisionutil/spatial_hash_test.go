package collisionutil

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

func TestSpatialHashQueryDeduplicatesEntries(t *testing.T) {
	hash := NewSpatialHash[string](10)
	entry := &Entry[string]{
		Value: "sprite",
		Box:   AABB{MinX: 0, MinY: 0, MaxX: 15, MaxY: 15},
	}
	hash.Insert(entry)

	results := hash.Query(AABB{MinX: 5, MinY: 5, MaxX: 12, MaxY: 12})
	if len(results) != 1 {
		t.Fatalf("expected 1 unique result, got %d", len(results))
	}
	if results[0].Value != "sprite" {
		t.Fatalf("unexpected query result: %q", results[0].Value)
	}
}
