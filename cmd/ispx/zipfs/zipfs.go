package zipfs

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

type ZipFs struct {
	files map[string]*zip.File
	root  string
}

type zipDirEntry struct {
	file *zip.File
}

func (zde *zipDirEntry) Name() string {
	return filepath.Base(zde.file.Name)
}

func (zde *zipDirEntry) IsDir() bool {
	return strings.HasSuffix(zde.file.Name, "/")
}

func (zde *zipDirEntry) Type() fs.FileMode {
	if zde.IsDir() {
		return fs.ModeDir
	}
	return 0
}

func (zde *zipDirEntry) Info() (fs.FileInfo, error) {
	return zde.file.FileInfo(), nil
}

func NewZipFsFromReader(reader *zip.Reader) *ZipFs {
	zf := &ZipFs{
		files: make(map[string]*zip.File),
	}

	for _, file := range reader.File {
		zf.files[file.Name] = file
	}

	return zf
}

func (zf *ZipFs) Chrooted(root string) *ZipFs {
	return &ZipFs{
		files: zf.files,
		root:  root,
	}
}

// Implement gop/parser/fsx.FileSystem:
//
//	type FileSystem interface {
//		ReadDir(dirname string) ([]fs.DirEntry, error)
//		ReadFile(filename string) ([]byte, error)
//		Join(elem ...string) string

//		// Base returns the last element of path.
//		// Trailing path separators are removed before extracting the last element.
//		// If the path is empty, Base returns ".".
//		// If the path consists entirely of separators, Base returns a single separator.
//		Base(filename string) string

//		// Abs returns an absolute representation of path.
//		Abs(path string) (string, error)
//	}

func (zf *ZipFs) ReadDir(dirname string) ([]fs.DirEntry, error) {
	dirname = path.Clean(path.Join(zf.root, dirname))
	if !strings.HasSuffix(dirname, "/") {
		dirname += "/"
	}
	if dirname == "/" {
		dirname = "./"
	}

	var dirEntries []fs.DirEntry

	for name, file := range zf.files {
		dir := path.Dir(strings.TrimSuffix(name, "/")) + "/"
		if dir != dirname {
			continue
		}

		dirEntries = append(dirEntries, &zipDirEntry{file})
	}

	if len(dirEntries) == 0 {
		return nil, fs.ErrNotExist
	}

	return dirEntries, nil
}

func (zf *ZipFs) ReadFile(filename string) ([]byte, error) {
	filename = path.Clean(path.Join(zf.root, filename))
	file, ok := zf.files[filename]
	if !ok {
		return nil, fs.ErrNotExist
	}

	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

func (z *ZipFs) Join(elem ...string) string {
	return path.Join(elem...)
}

// Base returns the last element of path.
// Trailing path separators are removed before extracting the last element.
// If the path is empty, Base returns ".".
// If the path consists entirely of separators, Base returns a single separator.
func (z *ZipFs) Base(filename string) string {
	return filepath.Base(filename)
}

// Abs returns an absolute representation of path.
func (z *ZipFs) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Implement spx/fs.Dir:
//
// type Dir interface {
// 	Open(file string) (io.ReadCloser, error)
// 	Close() error
// }

type readSeekCloser struct {
	*bytes.Reader
}

func (rsc *readSeekCloser) Close() error {
	return nil
}

func (z *ZipFs) Open(file string) (
	io.ReadCloser, error,
	// Although the interface requires io.ReadCloser, issues arise when using
	// io.ReadCloser with some file types in spx, which need Seek.
) {
	file = path.Clean(path.Join(z.root, file))
	f, ok := z.files[file]
	if !ok {
		return nil, fs.ErrNotExist
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return &readSeekCloser{bytes.NewReader(buf)}, nil
}

func (z *ZipFs) Close() error {
	return nil
}
