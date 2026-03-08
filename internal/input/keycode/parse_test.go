package keycode

import "testing"

func TestParseRecognizesAliases(t *testing.T) {
	tests := []struct {
		name string
		want int64
	}{
		{name: "A", want: KeyA},
		{name: "a", want: KeyA},
		{name: "Enter", want: KeyEnter},
		{name: "Return", want: KeyEnter},
		{name: "Ctrl", want: KeyControl},
		{name: "KP/", want: KeyKPDivide},
		{name: " ", want: KeySpace},
	}

	for _, tt := range tests {
		got, ok := Parse(tt.name)
		if !ok {
			t.Fatalf("Parse(%q) reported unknown key", tt.name)
		}
		if got != tt.want {
			t.Fatalf("Parse(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestParseRejectsUnknownKey(t *testing.T) {
	if _, ok := Parse("NotAKey"); ok {
		t.Fatal("Parse accepted an unknown key")
	}
}
