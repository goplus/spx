package filedetect

import (
	"os"
	"strings"

	"github.com/h2non/filetype"
)

// FileFormatResult represents the file format detection result.
type FileFormatResult struct {
	IsCorrect bool   // Whether the format detection was successful and supported
	Extension string // The detected file extension
}

var readFileByLen func(file string, len int) ([]byte, error)

func RegisterIoReader(onReadFile func(file string, len int) ([]byte, error)) {
	readFileByLen = onReadFile
}

func getBuffer(filepath string, len int) ([]byte, error) {
	// Read file header for detection
	var buffer []byte
	var err error

	if readFileByLen != nil {
		// Use registered reader function
		buffer, err = readFileByLen(filepath, len) // filetype needs max 262 bytes
		if err != nil {
			println(filepath, "header load failed1", err)

		}
		return buffer, err
	} else {
		// Use standard file reading
		file, err := os.Open(filepath)
		if err != nil {
			println(filepath, "header load failed2", err)
			return buffer, err
		}
		defer file.Close()

		buffer = make([]byte, len)
		n, err := file.Read(buffer)
		if err != nil && n == 0 {
			println(filepath, "header load failed3", err)
			return buffer, err
		}
		buffer = buffer[:n]
	}
	return buffer, err
}

// GetFileFormat gets the file format and its correctness
func GetFileFormat(filepath string) *FileFormatResult {
	// Extract file extension from filepath
	fileExt := ""
	if lastDot := strings.LastIndex(filepath, "."); lastDot != -1 {
		fileExt = strings.ToLower(filepath[lastDot:])
	}

	// Check svg
	buffer, err := getBuffer(filepath, 64)
	if err != nil {
		println(filepath, "header load failed", err)
		return &FileFormatResult{IsCorrect: false, Extension: ""}
	}

	if len(buffer) >= 5 {
		headerStr := strings.ToLower(string(buffer[:min(len(buffer), 64)]))
		if strings.Contains(headerStr, "<svg") || (strings.HasPrefix(headerStr, "<?xml") && strings.Contains(headerStr, "svg")) {
			isCorrect := strings.EqualFold(fileExt, ".svg")
			return &FileFormatResult{IsCorrect: isCorrect, Extension: ".svg"}
		}
	}

	buffer, err = getBuffer(filepath, 262)
	if err != nil {
		println(filepath, "header load failed", err)
		return &FileFormatResult{IsCorrect: false, Extension: ""}
	}
	// Detect file type using filetype library
	kind, err := filetype.Match(buffer)
	if err != nil || kind == filetype.Unknown {
		return &FileFormatResult{IsCorrect: false, Extension: ""}
	}

	// Get detected extension with dot prefix
	detectedExt := "." + kind.Extension

	// IsCorrect means: file extension matches detected format
	isCorrect := strings.EqualFold(fileExt, detectedExt)

	return &FileFormatResult{IsCorrect: isCorrect, Extension: detectedExt}
}
