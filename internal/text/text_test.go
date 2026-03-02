package text

import (
	"testing"
)

func TestIsASCII(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want bool
	}{
		{"empty string", "", true},
		{"ascii only", "hello world", true},
		{"with numbers", "abc123", true},
		{"with symbols", "hello!@#$%", true},
		{"chinese characters", "你好世界", false},
		{"mixed ascii and chinese", "hello你好", false},
		{"japanese characters", "こんにちは", false},
		{"emoji", "hello 😊", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isASCII(tt.str); got != tt.want {
				t.Errorf("isASCII(%q) = %v, want %v", tt.str, got, tt.want)
			}
		})
	}
}

func TestGetCharWidth(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want int
	}{
		{"ascii letter", 'a', singleCharWidth},
		{"ascii digit", '1', singleCharWidth},
		{"ascii space", ' ', singleCharWidth},
		{"ascii symbol", '!', singleCharWidth},
		{"chinese character", '你', doubleCharWidth},
		{"chinese character 2", '好', doubleCharWidth},
		{"non-ascii character", 'é', doubleCharWidth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCharWidth(tt.r); got != tt.want {
				t.Errorf("getCharWidth(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestCalculateWordWidth(t *testing.T) {
	tests := []struct {
		name string
		word string
		want int
	}{
		{"empty", "", 0},
		{"ascii word", "hello", 5},
		{"single char", "a", 1},
		{"chinese word", "你好", 4},
		{"mixed", "hi你", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateWordWidth(tt.word); got != tt.want {
				t.Errorf("calculateWordWidth(%q) = %v, want %v", tt.word, got, tt.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "empty string",
			input:    "",
			maxWidth: 10,
			want:     "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			maxWidth: 10,
			want:     "",
		},
		{
			name:     "single short word",
			input:    "hello",
			maxWidth: 10,
			want:     "hello",
		},
		{
			name:     "two words fit on one line",
			input:    "hello world",
			maxWidth: 12,
			want:     "hello world",
		},
		{
			name:     "two words need two lines",
			input:    "hello world",
			maxWidth: 8,
			want:     "hello\nworld",
		},
		{
			name:     "multiple words wrapping",
			input:    "the quick brown fox",
			maxWidth: 10,
			want:     "the quick\nbrown fox",
		},
		{
			name:     "word longer than maxWidth",
			input:    "supercalifragilistic is long",
			maxWidth: 10,
			want:     "supercalifragilistic\nis long",
		},
		{
			name:     "word longer than maxWidth at start",
			input:    "supercalifragilistic",
			maxWidth: 10,
			want:     "supercalifragilistic",
		},
		{
			name:     "chinese characters wrapping",
			input:    "你好世界测试文本",
			maxWidth: 6,
			want:     "你好世界\n测试文本\n",
		},
		{
			name:     "chinese short enough",
			input:    "你好",
			maxWidth: 10,
			want:     "你好",
		},
		{
			name:     "exact fit",
			input:    "ab cd",
			maxWidth: 5,
			want:     "ab cd",
		},
		{
			name:     "space counting",
			input:    "ab cd ef",
			maxWidth: 5,
			want:     "ab cd\nef",
		},
		{
			name:     "single character words",
			input:    "a b c d e",
			maxWidth: 3,
			want:     "a b\nc d\ne",
		},
		{
			name:     "long word between short words",
			input:    "hi superlongword bye",
			maxWidth: 5,
			want:     "hi\nsuperlongword\nbye",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitLines(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("SplitLines(%q, %d) =\n%q\nwant:\n%q", tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}
