package memfs

import (
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"time"
)

const (
	fileMode = fs.FileMode(0o444)
	dirMode  = fs.FileMode(0o444) | fs.ModeDir
)

// FS implements [fs.ReadDirFS] using a map of files.
type FS struct {
	files map[string][]byte
}

// New creates a new [FS].
func New(files map[string][]byte) *FS {
	return &FS{files: files}
}

// Open implements [fs.ReadDirFS].
func (fsys *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// Check if it's a file.
	if content, ok := fsys.files[name]; ok {
		return &file{
			name:    name,
			content: content,
		}, nil
	}

	// Check if it's a directory.
	if fsys.hasDir(name) {
		return &dir{
			fsys: fsys,
			name: name,
		}, nil
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// ReadDir implements [fs.ReadDirFS].
func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	prefix := ""
	if name != "." {
		prefix = name + "/"
	}

	// Map entry names to whether they are directories (true = dir, false = file).
	// If an entry appears both as a file and a directory prefix, it's a directory.
	entries := make(map[string]bool)
	for p := range fsys.files {
		after, ok := strings.CutPrefix(p, prefix)
		if !ok {
			continue
		}
		entryName, _, isDir := strings.Cut(after, "/")
		if entryName == "" {
			continue
		}
		if isDir || !entries[entryName] {
			entries[entryName] = isDir
		}
	}

	// Check if the directory exists. The root directory always exists.
	if name != "." && len(entries) == 0 {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}

	des := make([]fs.DirEntry, 0, len(entries))
	for _, entryName := range slices.Sorted(maps.Keys(entries)) {
		isDir := entries[entryName]
		de := &dirEntry{name: entryName}
		if isDir {
			de.mode = dirMode
		} else {
			de.mode = fileMode
			de.size = int64(len(fsys.files[prefix+entryName]))
		}
		des = append(des, de)
	}
	return des, nil
}

// Stat implements [fs.StatFS].
func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	// Check if it's a file.
	if content, ok := fsys.files[name]; ok {
		return &fileInfo{
			name: path.Base(name),
			size: int64(len(content)),
			mode: fileMode,
		}, nil
	}

	// Check if it's a directory.
	if fsys.hasDir(name) {
		return &fileInfo{
			name: path.Base(name),
			mode: dirMode,
		}, nil
	}

	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// hasDir reports whether name exists as a directory.
func (fsys *FS) hasDir(name string) bool {
	if name == "." {
		return true
	}
	prefix := name + "/"
	for p := range fsys.files {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// file implements [fs.File].
type file struct {
	name    string
	content []byte
	offset  int64
}

// Stat implements [fs.File].
func (f *file) Stat() (fs.FileInfo, error) {
	return &fileInfo{
		name: path.Base(f.name),
		size: int64(len(f.content)),
		mode: fileMode,
	}, nil
}

// Read implements [fs.File].
func (f *file) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.content)) {
		return 0, io.EOF
	}

	n := copy(b, f.content[f.offset:])
	f.offset += int64(n)
	return n, nil
}

// Close implements [fs.File].
func (f *file) Close() error {
	return nil
}

// dir implements [fs.ReadDirFile].
type dir struct {
	fsys    *FS
	name    string
	entries []fs.DirEntry
	offset  int
}

// Stat implements [fs.File].
func (d *dir) Stat() (fs.FileInfo, error) {
	return &fileInfo{
		name: path.Base(d.name),
		mode: dirMode,
	}, nil
}

// Read implements [fs.File].
func (d *dir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}

// Close implements [fs.File].
func (d *dir) Close() error {
	return nil
}

// ReadDir implements [fs.ReadDirFile].
func (d *dir) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.entries == nil {
		entries, err := d.fsys.ReadDir(d.name)
		if err != nil {
			return nil, err
		}
		d.entries = entries
	}

	if n <= 0 {
		entries := d.entries[d.offset:]
		d.offset = len(d.entries)
		return entries, nil
	}

	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}

	end := min(d.offset+n, len(d.entries))
	entries := d.entries[d.offset:end]
	d.offset = end
	return entries, nil
}

// fileInfo implements [fs.FileInfo].
type fileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() fs.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fi *fileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi *fileInfo) Sys() any           { return nil }

// dirEntry implements [fs.DirEntry].
type dirEntry struct {
	name string
	size int64
	mode fs.FileMode
}

func (de *dirEntry) Name() string      { return de.name }
func (de *dirEntry) IsDir() bool       { return de.mode.IsDir() }
func (de *dirEntry) Type() fs.FileMode { return de.mode.Type() }
func (de *dirEntry) Info() (fs.FileInfo, error) {
	return &fileInfo{
		name: de.name,
		size: de.size,
		mode: de.mode,
	}, nil
}
