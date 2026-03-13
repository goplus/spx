package engine

import "testing"

func TestSpriteSyncBufferSerializeReusesScratch(t *testing.T) {
	buffer := NewSpriteSyncBuffer(1)
	buffer.Add(1, 2, 3, 4, 5, 6, 7, 8, true)
	buffer.AddDelete(9)

	first := buffer.Serialize()
	if len(first) == 0 {
		t.Fatal("Serialize returned an empty buffer")
	}
	firstPtr := &first[0]

	buffer.Clear()
	buffer.Add(10, 11, 12, 13, 14, 15, 16, 17, false)

	second := buffer.Serialize()
	if len(second) == 0 {
		t.Fatal("Serialize returned an empty buffer after reuse")
	}
	if firstPtr != &second[0] {
		t.Fatal("Serialize allocated a new backing array instead of reusing scratch storage")
	}
	if got, want := len(second), 2+SyncFieldsPerSprite; got != want {
		t.Fatalf("Serialize len = %d, want %d", got, want)
	}
	if got, want := cap(second), len(second); got != want {
		t.Fatalf("Serialize cap = %d, want %d", got, want)
	}
	if got, want := second[0], float32(1); got != want {
		t.Fatalf("header updateCount = %v, want %v", got, want)
	}
	if got, want := second[1], float32(0); got != want {
		t.Fatalf("header deleteCount = %v, want %v", got, want)
	}
	if got, want := second[2], float32(10); got != want {
		t.Fatalf("sprite id = %v, want %v", got, want)
	}
}

func TestVisualSyncBufferSerializeReusesScratch(t *testing.T) {
	buffer := NewVisualSyncBuffer(1)
	buffer.AddFull(1, 2, 3, true, [4]float64{4, 5, 6, 7}, true)

	first := buffer.Serialize()
	if len(first) == 0 {
		t.Fatal("Serialize returned an empty buffer")
	}
	firstPtr := &first[0]

	buffer.Clear()
	buffer.AddRenderScale(8, 9)

	second := buffer.Serialize()
	if len(second) == 0 {
		t.Fatal("Serialize returned an empty buffer after reuse")
	}
	if firstPtr != &second[0] {
		t.Fatal("Serialize allocated a new backing array instead of reusing scratch storage")
	}
	if got, want := len(second), 1+VisualFieldsPerSprite; got != want {
		t.Fatalf("Serialize len = %d, want %d", got, want)
	}
	if got, want := cap(second), len(second); got != want {
		t.Fatalf("Serialize cap = %d, want %d", got, want)
	}
	if got, want := second[0], float32(1); got != want {
		t.Fatalf("header count = %v, want %v", got, want)
	}
	if got, want := second[1], float32(8); got != want {
		t.Fatalf("sprite id = %v, want %v", got, want)
	}
}

func TestEnsureFloat32BufferSizeAddsHeadroom(t *testing.T) {
	buf := ensureFloat32BufferSize(nil, 17)
	if got, want := len(buf), 17; got != want {
		t.Fatalf("len(buf) = %d, want %d", got, want)
	}
	if got := cap(buf); got <= len(buf) {
		t.Fatalf("cap(buf) = %d, want > %d", got, len(buf))
	}
}
