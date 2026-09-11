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

// Package text provides text processing utilities for SPX.
package text

import (
	"strings"
	"unicode"
)

const (
	singleCharWidth = 1 // Printable ASCII.
	doubleCharWidth = 2 // Non-ASCII and non-printable characters.
)

// lineWriter wraps a strings.Builder with line-width tracking for word wrapping.
type lineWriter struct {
	buf       strings.Builder
	lineWidth int
	maxWidth  int
}

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

	if w.lineWidth+wordWidth > w.maxWidth {
		w.newLine()
	}

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

func isASCII(str string) bool {
	for _, r := range str {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// getCharWidth treats non-ASCII and non-printable runes as double-width.
func getCharWidth(r rune) int {
	if r > unicode.MaxASCII || !unicode.IsPrint(r) {
		return doubleCharWidth
	}
	return singleCharWidth
}

func calculateWordWidth(word string) int {
	width := 0
	for _, r := range word {
		width += getCharWidth(r)
	}
	return width
}
