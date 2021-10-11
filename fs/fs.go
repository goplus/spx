package fs

import (
	"errors"
	"io"
	"strings"
)

// -------------------------------------------------------------------------------------

type Dir interface {
	Open(file string) (io.ReadCloser, error)
	Close() error
}

func SplitSchema(path string) (schema, file string) {
	idx := strings.IndexAny(path, ":/\\ ")
	if idx < 0 || path[idx] != ':' {
		return "", path
	}
	schema, file = path[:idx], path[idx+1:]
	file = strings.TrimPrefix(file, "//")
	return
}

// -------------------------------------------------------------------------------------

type OpenFunc = func(file string) (Dir, error)

var (
	openSchemas = map[string]OpenFunc{}
)

func RegisterSchema(schema string, open OpenFunc) {
	openSchemas[schema] = open
}

func Open(path string) (Dir, error) {
	schema, file := SplitSchema(path)
	if open, ok := openSchemas[schema]; ok {
		return open(file)
	}
	return nil, errors.New("fs.Open: unsupported schema - " + schema)
}

// -------------------------------------------------------------------------------------
