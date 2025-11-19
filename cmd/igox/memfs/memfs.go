package memfs

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type MemFs struct {
	mu    sync.RWMutex
	files map[string][]byte
	root  string
}

type memDirEntry struct {
	name  string
	isDir bool
}

func (mde *memDirEntry) Name() string {
	return mde.name
}

func (mde *memDirEntry) IsDir() bool {
	return mde.isDir
}

func (mde *memDirEntry) Type() fs.FileMode {
	if mde.isDir {
		return fs.ModeDir
	}
	return 0
}

func (mde *memDirEntry) Info() (fs.FileInfo, error) {
	return nil, errors.New("not implemented")
}

// NewMemFs creates a new in-memory file system
func NewMemFs(files map[string][]byte) *MemFs {
	return &MemFs{
		files: files,
	}
}

// Chroot creates a new MemFs with a different root
func (m *MemFs) Chroot(root string) (*MemFs, error) {
	clean := path.Clean(root)
	clean = strings.TrimPrefix(clean, "/")
	return &MemFs{
		files: m.files,
		root:  clean,
	}, nil
}

// ReadDir lists directory entries under dirname
func (m *MemFs) ReadDir(dirname string) ([]fs.DirEntry, error) {
	dirname = path.Clean(path.Join(m.root, dirname))
	if !strings.HasSuffix(dirname, "/") {
		dirname += "/"
	}
	if dirname == "/" {
		dirname = ""
	}

	entries := make(map[string]bool) // value indicates if it's a directory
	m.mu.RLock()
	for name := range m.files {
		if !strings.HasPrefix(name, dirname) {
			continue
		}
		rel := strings.TrimPrefix(name, dirname)
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			// If there are multiple parts, this is a subdirectory
			isDir := len(parts) > 1
			if existing, ok := entries[parts[0]]; ok {
				// If already exists and either is a directory, mark as directory
				entries[parts[0]] = existing || isDir
			} else {
				entries[parts[0]] = isDir
			}
		}
	}
	m.mu.RUnlock()

	if len(entries) == 0 {
		return nil, fs.ErrNotExist
	}

	var dirEntries []fs.DirEntry
	for name, isDir := range entries {
		dirEntries = append(dirEntries, &memDirEntry{
			name:  name,
			isDir: isDir,
		})
	}

	// Sort entries
	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	return dirEntries, nil
}

// ReadFile returns the file content for filename
func (m *MemFs) ReadFile(filename string) ([]byte, error) {
	filename = path.Clean(path.Join(m.root, filename))
	m.mu.RLock()
	data, ok := m.files[filename]
	m.mu.RUnlock()
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
	m.mu.RLock()
	data, ok := m.files[file]
	m.mu.RUnlock()
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
	m.mu.Lock()
	m.files[filename] = data
	m.mu.Unlock()
}

// RemoveFile removes a file from the file system
func (m *MemFs) RemoveFile(filename string) {
	filename = path.Clean(filename)
	m.mu.Lock()
	delete(m.files, filename)
	m.mu.Unlock()
}
