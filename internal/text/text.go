package text

import (
	"strings"
	"unicode"
)

func isAscii(str string) bool {
	for _, r := range str {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func getCharLen(r rune) int {
	if unicode.Is(unicode.Han, r) || !unicode.IsPrint(r) || r > unicode.MaxASCII {
		return 2
	}
	return 1
}
func calculateWordLength(word string) int {
	length := 0
	for _, r := range word {
		length += getCharLen(r)
	}
	return length
}

// splitString splits the input string into lines with a maximum placeholder width n
func SplitLines(input string, n int) string {
	words := strings.Fields(input)
	var result strings.Builder
	lineLength := 0

	for i, word := range words {
		wordLength := calculateWordLength(word)
		if isAscii(word) {
			// If the word length is greater than n, place it on a separate line
			if wordLength > n {
				if lineLength > 0 {
					result.WriteString("\n")
				}
				result.WriteString(word)
				if i < len(words)-1 {
					result.WriteString("\n")
				}
				lineLength = 0
				continue
			}

			// If adding the word exceeds the line length n, start a new line
			if lineLength+wordLength > n {
				result.WriteString("\n")
				lineLength = 0
			}

			// Add a space if the line already has content
			if lineLength > 0 {
				result.WriteString(" ") // A space counts as one placeholder
				lineLength++
			}

			result.WriteString(word)
			lineLength += wordLength
		} else {
			for _, c := range word {
				length := getCharLen(c)
				result.WriteRune(c)
				lineLength += length
				if lineLength > n {
					result.WriteString("\n")
					lineLength = 0
				}
			}
		}
	}
	return result.String()
}
