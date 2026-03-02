// Package text provides text processing utilities for SPX.
package text

import (
	"strings"
	"unicode"
)

// Character width constants for display calculation
const (
	singleCharWidth = 1 // Width for ASCII printable characters
	doubleCharWidth = 2 // Width for CJK characters and other wide characters
)

// SplitLines splits the input string into lines with a maximum display width.
// It handles both ASCII words (split by spaces) and CJK characters (split by character).
func SplitLines(input string, maxWidth int) string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return ""
	}

	w := lineWriter{maxWidth: maxWidth}

	for i, word := range words {
		if isASCII(word) {
			w.writeASCIIWord(word, i < len(words)-1)
		} else {
			w.writeCJKWord(word)
		}
	}

	return w.buf.String()
}

// isASCII checks if a string contains only ASCII characters.
func isASCII(str string) bool {
	for _, r := range str {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// getCharWidth returns the display width of a rune.
// CJK characters, non-printable characters, and non-ASCII characters are treated as double-width.
func getCharWidth(r rune) int {
	if unicode.Is(unicode.Han, r) || !unicode.IsPrint(r) || r > unicode.MaxASCII {
		return doubleCharWidth
	}
	return singleCharWidth
}

// calculateWordWidth calculates the total display width of a word.
func calculateWordWidth(word string) int {
	width := 0
	for _, r := range word {
		width += getCharWidth(r)
	}
	return width
}

// lineWriter wraps a strings.Builder with line-width tracking for word wrapping.
type lineWriter struct {
	buf       strings.Builder
	lineWidth int
	maxWidth  int
}

// newLine starts a new line and resets the current line width.
func (w *lineWriter) newLine() {
	w.buf.WriteString("\n")
	w.lineWidth = 0
}

// writeASCIIWord writes an ASCII word with proper wrapping and spacing.
// If hasMore is true and the word exceeds maxWidth, a trailing newline is added.
func (w *lineWriter) writeASCIIWord(word string, hasMore bool) {
	wordWidth := calculateWordWidth(word)

	// If the word is longer than maxWidth, place it on its own line
	if wordWidth > w.maxWidth {
		if w.lineWidth > 0 {
			w.newLine()
		}
		w.buf.WriteString(word)
		if hasMore {
			w.newLine()
		}
		return
	}

	// If adding the word would exceed maxWidth, start a new line
	if w.lineWidth+wordWidth > w.maxWidth {
		w.newLine()
	}

	// Add space separator if line has content
	if w.lineWidth > 0 {
		w.buf.WriteString(" ")
		w.lineWidth += singleCharWidth
	}

	w.buf.WriteString(word)
	w.lineWidth += wordWidth
}

// writeCJKWord writes a CJK word character by character with wrapping.
func (w *lineWriter) writeCJKWord(word string) {
	for _, char := range word {
		charWidth := getCharWidth(char)
		w.buf.WriteRune(char)
		w.lineWidth += charWidth

		if w.lineWidth > w.maxWidth {
			w.newLine()
		}
	}
}
