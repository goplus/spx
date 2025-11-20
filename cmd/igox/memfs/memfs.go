package memfs

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MemFs is an in-memory file system that stores files as a map of paths to byte slices.
// It is safe for concurrent use. Chrooted instances share the underlying file map.
type MemFs struct {
	files map[string][]byte
	root  string
}

// memDirEntry implements the fs.DirEntry interface
type memDirEntry struct {
	name  string
	isDir bool
	size  int64
}

func (e *memDirEntry) Name() string {
	return e.name
}

func (e *memDirEntry) IsDir() bool {
	return e.isDir
}

func (e *memDirEntry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}
	return 0
}

func (e *memDirEntry) Info() (fs.FileInfo, error) {
	return &memFileInfo{
		name:  e.name,
		size:  e.size,
		isDir: e.isDir,
	}, nil
}

// memFileInfo implements the fs.FileInfo interface
type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi *memFileInfo) Name() string { return fi.name }
func (fi *memFileInfo) Size() int64  { return fi.size }
func (fi *memFileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (fi *memFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *memFileInfo) IsDir() bool        { return fi.isDir }
func (fi *memFileInfo) Sys() interface{}   { return nil }

// NewMemFs creates a new in-memory file system
func NewMemFs(files map[string][]byte) *MemFs {
	return &MemFs{
		files: files,
	}
}

// Chroot creates a new MemFs with a different root
func (m *MemFs) Chroot(root string) (*MemFs, error) {
	return &MemFs{
		files: m.files,
		root:  root,
	}, nil
}

// ReadDir lists directory entries under dirname
func (m *MemFs) ReadDir(dirname string) ([]fs.DirEntry, error) {
	dirname = path.Clean(path.Join(m.root, dirname))
	if !strings.HasSuffix(dirname, "/") {
		dirname += "/"
	}
	if dirname == "/" {
		dirname = "./"
	}

	// used for deduplication
	seen := make(map[string]*memDirEntry)

	for name, content := range m.files {
		// skip files that are not under the target directory
		if !strings.HasPrefix(name, dirname) && dirname != "./" {
			continue
		}

		// special handling for the root directory
		var relativePath string
		if dirname == "./" {
			relativePath = name
		} else {
			relativePath = strings.TrimPrefix(name, dirname)
		}

		// if the relative path is empty, skip (it's the directory itself)
		if relativePath == "" {
			continue
		}

		// check whether it's a direct child
		parts := strings.SplitN(relativePath, "/", 2)
		if len(parts) == 0 {
			continue
		}

		entryName := parts[0]

		// determine whether it's a file or a directory
		isDir := len(parts) > 1 && parts[1] != ""

		if existing, exists := seen[entryName]; exists {
			// if already exists and currently determined to be a directory, update to directory
			if isDir && !existing.isDir {
				existing.isDir = true
			}
		} else {
			var size int64
			if !isDir {
				size = int64(len(content))
			}

			seen[entryName] = &memDirEntry{
				name:  entryName,
				isDir: isDir,
				size:  size,
			}
		}
	}

	if len(seen) == 0 {
		return nil, fs.ErrNotExist
	}
	// convert map entries to a slice and sort
	dirEntries := make([]fs.DirEntry, 0, len(seen))
	for _, entry := range seen {
		dirEntries = append(dirEntries, entry)
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	return dirEntries, nil
}

// ReadFile returns the file content for filename
func (m *MemFs) ReadFile(filename string) ([]byte, error) {
	filename = path.Clean(path.Join(m.root, filename))
	data, ok := m.files[filename]
	if !ok {
		return nil, fs.ErrNotExist
	}
	// Return a copy to prevent external modification
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

// Join joins path elements
func (m *MemFs) Join(elem ...string) string {
	return path.Join(elem...)
}

// Base returns the last element of path
func (m *MemFs) Base(filename string) string {
	return filepath.Base(filename)
}

// Abs returns an absolute representation of path
func (m *MemFs) Abs(p string) (string, error) {
	return filepath.Abs(p)
}

type readSeekCloser struct {
	*bytes.Reader
}

func (rsc *readSeekCloser) Close() error {
	return nil
}

// Open opens a file for reading
func (m *MemFs) Open(file string) (io.ReadCloser, error) {
	file = path.Clean(path.Join(m.root, file))
	data, ok := m.files[file]
	if !ok {
		return nil, fs.ErrNotExist
	}

	// Return a new reader with Seek support
	return &readSeekCloser{bytes.NewReader(data)}, nil
}

// Close closes the file system (no-op for MemFs)
func (m *MemFs) Close() error {
	return nil
}

// AddFile adds or updates a file in the file system
func (m *MemFs) AddFile(filename string, data []byte) {
	filename = path.Clean(filename)
	m.files[filename] = data
}

// RemoveFile removes a file from the file system
func (m *MemFs) RemoveFile(filename string) {
	filename = path.Clean(filename)
	delete(m.files, filename)
}
