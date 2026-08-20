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

package projectbundle

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const (
	// DefaultMaxEntries limits the number of regular files in a bundle.
	DefaultMaxEntries = 10_000
	// DefaultMaxFileBytes limits the uncompressed size of one file.
	DefaultMaxFileBytes int64 = 64 << 20
	// DefaultMaxTotalBytes limits the aggregate uncompressed input size.
	DefaultMaxTotalBytes int64 = 256 << 20
	// DefaultMaxArchiveBytes limits bytes written for the ZIP container.
	DefaultMaxArchiveBytes int64 = 512 << 20
)

var (
	// ErrInvalidPath reports an absolute, traversing, or non-canonical member path.
	ErrInvalidPath = errors.New("projectbundle: invalid relative path")
	// ErrUnsafeFile reports a symlink, non-regular file, or file changed while read.
	ErrUnsafeFile = errors.New("projectbundle: unsafe file")
	// ErrCollision reports duplicate, Unicode-canonical, or case-folded entry names.
	ErrCollision = errors.New("projectbundle: entry name collision")
	// ErrLimit reports an entry-count or byte-size limit violation.
	ErrLimit = errors.New("projectbundle: limit exceeded")
)

var (
	canonicalZipTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	unicodeFold      = cases.Fold()
)

// Config is the complete v1 allowlist for a project bundle.
//
// ProjectFiles and PackDir are slash-separated paths below ProjectDir.
// ProjectFiles must include every referenced project resource outside PackDir.
// PackDir is optional; when set, all of its regular files are included with
// PackDir retained as their archive prefix. IncludeConfig explicitly includes
// ProjectDir/.config. When ConfigBytes is non-nil, those bytes are included
// instead of reopening ProjectDir/.config; a non-nil empty slice represents an
// empty file. Output and FinalOutput are not written by this package; they are
// checked to ensure a compiler output cannot be recursively collected from
// PackDir.
type Config struct {
	ProjectDir    string
	ProjectFiles  []string
	IncludeConfig bool
	ConfigBytes   []byte
	PackDir       string
	Output        string
	FinalOutput   string
	Limits        Limits
}

// Limits bounds collection memory and ZIP output. A zero field selects its
// DefaultMax* value. Negative values are invalid.
type Limits struct {
	MaxEntries      int
	MaxFileBytes    int64
	MaxTotalBytes   int64
	MaxArchiveBytes int64
}

// Entry describes a collected archive member without exposing mutable data.
type Entry struct {
	Name string
	Size int64
}

// Digest is the SHA-256 of the complete canonical ZIP bytes.
type Digest [sha256.Size]byte

// String returns the lowercase hexadecimal SHA-256.
func (d Digest) String() string {
	return hex.EncodeToString(d[:])
}

// MarshalText implements encoding.TextMarshaler.
func (d Digest) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

type collectedEntry struct {
	name string
	data []byte
}

// Bundle is an immutable in-memory snapshot of collected source files.
// Source paths are closed before Collect returns, so later renames or metadata
// changes cannot change this bundle's archive bytes.
type Bundle struct {
	entries []collectedEntry
	limits  resolvedLimits
	total   int64
}

// Collect validates cfg, securely reads its allowlisted regular files, and
// returns an immutable bundle. It neither creates nor removes user files.
func Collect(cfg Config) (*Bundle, error) {
	limits, err := resolveLimits(cfg.Limits)
	if err != nil {
		return nil, err
	}
	if cfg.ProjectDir == "" {
		return nil, fmt.Errorf("%w: ProjectDir is empty", ErrInvalidPath)
	}

	projectRootObservation, err := observeRoot(cfg.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("projectbundle: observe ProjectDir: %w", err)
	}
	projectDir := projectRootObservation.path

	projectFiles := make([]string, len(cfg.ProjectFiles))
	for i, name := range cfg.ProjectFiles {
		projectFiles[i], err = validateRelativePath(name, "ProjectFiles")
		if err != nil {
			return nil, err
		}
	}

	packDir := ""
	if cfg.PackDir != "" {
		packDir, err = validateRelativePath(cfg.PackDir, "PackDir")
		if err != nil {
			return nil, err
		}
	}

	var packRoot string
	if packDir != "" {
		packRoot = filepath.Join(projectDir, filepath.FromSlash(packDir))
	}
	if err := validateOutputs(projectDir, cfg.Output, cfg.FinalOutput, packRoot); err != nil {
		return nil, err
	}

	projectRoot, err := openSafeRoot(projectDir, projectRootObservation.info)
	if err != nil {
		return nil, fmt.Errorf("projectbundle: open ProjectDir %q: %w", projectDir, err)
	}
	defer projectRoot.Close()

	collector := newCollector(limits)
	for _, name := range projectFiles {
		if err := collector.addFile(projectRoot, name, name); err != nil {
			return nil, err
		}
	}
	if cfg.IncludeConfig {
		var err error
		if cfg.ConfigBytes != nil {
			err = collector.addData(".config", cfg.ConfigBytes)
		} else {
			err = collector.addFile(projectRoot, ".config", ".config")
		}
		if err != nil {
			return nil, err
		}
	}
	if packDir != "" {
		root, err := projectRoot.OpenDir(filepath.FromSlash(packDir))
		if err != nil {
			return nil, fmt.Errorf("projectbundle: open PackDir %q: %w", packDir, err)
		}
		if err := collector.addTree(root, packDir); err != nil {
			root.Close()
			return nil, err
		}
		if err := root.Close(); err != nil {
			return nil, fmt.Errorf("projectbundle: close PackDir %q: %w", packDir, err)
		}
	}
	sort.Slice(collector.entries, func(i, j int) bool {
		return collector.entries[i].name < collector.entries[j].name
	})
	return &Bundle{entries: collector.entries, limits: limits, total: collector.total}, nil
}

// Archive is a convenience wrapper around Collect and Bundle.Bytes.
func Archive(cfg Config) ([]byte, Digest, error) {
	bundle, err := Collect(cfg)
	if err != nil {
		return nil, Digest{}, err
	}
	return bundle.Bytes()
}

// WriteArchive is a convenience wrapper around Collect and Bundle.WriteZIP.
func WriteArchive(w io.Writer, cfg Config) (Digest, error) {
	bundle, err := Collect(cfg)
	if err != nil {
		return Digest{}, err
	}
	return bundle.WriteZIP(w)
}

// Entries returns sorted metadata for the collected files.
func (b *Bundle) Entries() []Entry {
	if b == nil {
		return nil
	}
	entries := make([]Entry, len(b.entries))
	for i, entry := range b.entries {
		entries[i] = Entry{Name: entry.name, Size: int64(len(entry.data))}
	}
	return entries
}

// TotalBytes returns the aggregate uncompressed size of collected files.
func (b *Bundle) TotalBytes() int64 {
	if b == nil {
		return 0
	}
	return b.total
}

// Bytes returns the canonical ZIP bytes and their SHA-256 digest.
func (b *Bundle) Bytes() ([]byte, Digest, error) {
	var buffer bytes.Buffer
	digest, err := b.WriteZIP(&buffer)
	if err != nil {
		return nil, Digest{}, err
	}
	return buffer.Bytes(), digest, nil
}

// WriteZIP writes the canonical ZIP to w and returns its SHA-256. It does not
// close w. An error may leave a partial archive in w.
func (b *Bundle) WriteZIP(w io.Writer) (Digest, error) {
	if b == nil {
		return Digest{}, errors.New("projectbundle: nil Bundle")
	}
	if w == nil {
		return Digest{}, errors.New("projectbundle: nil writer")
	}

	hasher := sha256.New()
	limited := &archiveLimitWriter{writer: w, remaining: b.limits.maxArchiveBytes}
	writer := zip.NewWriter(io.MultiWriter(limited, hasher))
	writer.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})

	for _, entry := range b.entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetModTime(canonicalZipTime)
		header.SetMode(0o644)
		output, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return Digest{}, fmt.Errorf("projectbundle: create ZIP entry %q: %w", entry.name, err)
		}
		if _, err := output.Write(entry.data); err != nil {
			_ = writer.Close()
			return Digest{}, fmt.Errorf("projectbundle: write ZIP entry %q: %w", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return Digest{}, fmt.Errorf("projectbundle: close ZIP: %w", err)
	}

	var digest Digest
	copy(digest[:], hasher.Sum(nil))
	return digest, nil
}

type resolvedLimits struct {
	maxEntries      int
	maxFileBytes    int64
	maxTotalBytes   int64
	maxArchiveBytes int64
}

func resolveLimits(limits Limits) (resolvedLimits, error) {
	resolved := resolvedLimits{
		maxEntries:      limits.MaxEntries,
		maxFileBytes:    limits.MaxFileBytes,
		maxTotalBytes:   limits.MaxTotalBytes,
		maxArchiveBytes: limits.MaxArchiveBytes,
	}
	if resolved.maxEntries == 0 {
		resolved.maxEntries = DefaultMaxEntries
	}
	if resolved.maxFileBytes == 0 {
		resolved.maxFileBytes = DefaultMaxFileBytes
	}
	if resolved.maxTotalBytes == 0 {
		resolved.maxTotalBytes = DefaultMaxTotalBytes
	}
	if resolved.maxArchiveBytes == 0 {
		resolved.maxArchiveBytes = DefaultMaxArchiveBytes
	}
	if resolved.maxEntries < 0 || resolved.maxFileBytes < 0 || resolved.maxTotalBytes < 0 || resolved.maxArchiveBytes < 0 {
		return resolvedLimits{}, fmt.Errorf("%w: limits must not be negative", ErrLimit)
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if resolved.maxFileBytes == maxInt64 || resolved.maxTotalBytes == maxInt64 {
		return resolvedLimits{}, fmt.Errorf("%w: file and total byte limits must be less than MaxInt64", ErrLimit)
	}
	return resolved, nil
}

type collector struct {
	limits    resolvedLimits
	entries   []collectedEntry
	total     int64
	exact     map[string]string
	canonical map[string]string
	folded    map[string]string
}

func newCollector(limits resolvedLimits) *collector {
	return &collector{
		limits:    limits,
		exact:     make(map[string]string),
		canonical: make(map[string]string),
		folded:    make(map[string]string),
	}
}

func (c *collector) addFile(root *safeDir, sourcePath, archiveName string) error {
	archiveName, err := validateRelativePath(archiveName, "archive entry")
	if err != nil {
		return err
	}
	if err := c.reserveName(archiveName); err != nil {
		return err
	}
	if len(c.entries) >= c.limits.maxEntries {
		return fmt.Errorf("%w: more than %d entries", ErrLimit, c.limits.maxEntries)
	}

	file, err := root.OpenFile(filepath.FromSlash(sourcePath))
	if err != nil {
		return fmt.Errorf("projectbundle: open source %q: %w", sourcePath, err)
	}
	defer file.Close()

	data, err := readRegularFile(file, sourcePath, c.limits.maxFileBytes, c.limits.maxTotalBytes-c.total)
	if err != nil {
		return err
	}
	if int64(len(data)) > c.limits.maxTotalBytes-c.total {
		return fmt.Errorf("%w: total input exceeds %d bytes", ErrLimit, c.limits.maxTotalBytes)
	}
	c.total += int64(len(data))
	c.entries = append(c.entries, collectedEntry{name: archiveName, data: data})
	return nil
}

func (c *collector) addData(archiveName string, data []byte) error {
	archiveName, err := validateRelativePath(archiveName, "archive entry")
	if err != nil {
		return err
	}
	if err := c.reserveName(archiveName); err != nil {
		return err
	}
	if len(c.entries) >= c.limits.maxEntries {
		return fmt.Errorf("%w: more than %d entries", ErrLimit, c.limits.maxEntries)
	}
	size := int64(len(data))
	if size > c.limits.maxFileBytes {
		return fmt.Errorf("%w: source %q exceeds %d bytes", ErrLimit, archiveName, c.limits.maxFileBytes)
	}
	if size > c.limits.maxTotalBytes-c.total {
		return fmt.Errorf("%w: total input exceeds %d bytes", ErrLimit, c.limits.maxTotalBytes)
	}
	contents := append([]byte(nil), data...)
	if data != nil && contents == nil {
		contents = []byte{}
	}
	c.total += size
	c.entries = append(c.entries, collectedEntry{name: archiveName, data: contents})
	return nil
}

func (c *collector) addTree(root *safeDir, prefix string) error {
	entries, err := root.ReadDir()
	if err != nil {
		return fmt.Errorf("projectbundle: read PackDir %q: %w", prefix, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := prefix + "/" + entry.Name()
		if _, err := validateRelativePath(name, "PackDir entry"); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: PackDir entry %q is a symbolic link", ErrUnsafeFile, name)
		}
		if entry.IsDir() {
			child, err := root.OpenDir(entry.Name())
			if err != nil {
				return fmt.Errorf("projectbundle: open PackDir directory %q: %w", name, err)
			}
			err = c.addTree(child, name)
			closeErr := child.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return fmt.Errorf("projectbundle: close PackDir directory %q: %w", name, closeErr)
			}
			continue
		}
		if err := c.addFile(root, entry.Name(), name); err != nil {
			return err
		}
	}
	return nil
}

func (c *collector) reserveName(name string) error {
	if previous, ok := c.exact[name]; ok {
		return fmt.Errorf("%w: duplicate path %q (already declared as %q)", ErrCollision, name, previous)
	}
	canonical := norm.NFC.String(name)
	if previous, ok := c.canonical[canonical]; ok {
		return fmt.Errorf("%w: Unicode-canonical paths %q and %q", ErrCollision, previous, name)
	}
	folded := unicodeFold.String(canonical)
	if previous, ok := c.folded[folded]; ok {
		return fmt.Errorf("%w: case-folded paths %q and %q", ErrCollision, previous, name)
	}
	c.exact[name] = name
	c.canonical[canonical] = name
	c.folded[folded] = name
	return nil
}

func readRegularFile(file *os.File, name string, maxFileBytes, remainingTotalBytes int64) ([]byte, error) {
	before, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("projectbundle: fstat source %q: %w", name, err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: source %q has mode %s", ErrUnsafeFile, name, before.Mode())
	}
	if before.Size() < 0 || before.Size() > maxFileBytes {
		return nil, fmt.Errorf("%w: source %q exceeds %d bytes", ErrLimit, name, maxFileBytes)
	}
	if before.Size() > remainingTotalBytes {
		return nil, fmt.Errorf("%w: source %q exceeds remaining total allowance of %d bytes", ErrLimit, name, remainingTotalBytes)
	}
	readLimit := min(maxFileBytes, remainingTotalBytes)

	data, err := io.ReadAll(io.LimitReader(file, readLimit+1))
	if err != nil {
		return nil, fmt.Errorf("projectbundle: read source %q: %w", name, err)
	}
	if int64(len(data)) > readLimit {
		return nil, fmt.Errorf("%w: source %q grew beyond its byte allowance", ErrLimit, name)
	}
	after, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("projectbundle: second fstat source %q: %w", name, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() ||
		!before.ModTime().Equal(after.ModTime()) || int64(len(data)) != after.Size() {
		return nil, fmt.Errorf("%w: source %q changed while it was read", ErrUnsafeFile, name)
	}
	return data, nil
}

func validateRelativePath(name, field string) (string, error) {
	if name == "" || name == "." || !utf8.ValidString(name) || strings.IndexByte(name, 0) >= 0 {
		return "", fmt.Errorf("%w: %s %q", ErrInvalidPath, field, name)
	}
	if strings.ContainsRune(name, '\\') || path.IsAbs(name) || filepath.IsAbs(name) || looksLikeWindowsAbsolutePath(name) {
		return "", fmt.Errorf("%w: %s %q must be slash-separated and relative", ErrInvalidPath, field, name)
	}
	if path.Clean(name) != name {
		return "", fmt.Errorf("%w: %s %q is not clean", ErrInvalidPath, field, name)
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || component == "." || component == ".." {
			return "", fmt.Errorf("%w: %s %q contains an invalid component", ErrInvalidPath, field, name)
		}
		if err := validatePortableComponent(component); err != nil {
			return "", fmt.Errorf("%w: %s %q: %v", ErrInvalidPath, field, name, err)
		}
	}
	return name, nil
}

func validatePortableComponent(component string) error {
	if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
		return errors.New("component ends in a dot or space")
	}
	if strings.ContainsAny(component, `<>:"|?*`) {
		return errors.New("component contains a Windows-reserved character")
	}
	for _, character := range component {
		if character < 0x20 {
			return errors.New("component contains a control character")
		}
	}

	base := component
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	base = strings.ToUpper(strings.TrimRight(base, " ."))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$":
		return errors.New("component uses a reserved DOS device name")
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return errors.New("component uses a reserved DOS device name")
	}
	for _, prefix := range []string{"COM", "LPT"} {
		for _, digit := range []string{"¹", "²", "³"} {
			if base == prefix+digit {
				return errors.New("component uses a reserved DOS device name")
			}
		}
	}
	return nil
}

func looksLikeWindowsAbsolutePath(name string) bool {
	if strings.HasPrefix(name, "//") || strings.HasPrefix(name, `\\`) {
		return true
	}
	return len(name) >= 3 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) && name[1] == ':' && (name[2] == '/' || name[2] == '\\')
}

func validateOutputs(projectDir, output, finalOutput, packRoot string) error {
	var roots []string
	if packRoot != "" {
		roots = append(roots, packRoot)
	}

	canonicalRoots := make([]string, len(roots))
	for i, root := range roots {
		canonical, err := canonicalFilesystemPath(root)
		if err != nil {
			return fmt.Errorf("projectbundle: canonicalize input root %q: %w", root, err)
		}
		canonicalRoots[i] = canonical
	}
	for _, item := range []struct {
		field string
		value string
	}{
		{field: "Output", value: output},
		{field: "FinalOutput", value: finalOutput},
	} {
		if item.value == "" {
			continue
		}
		candidate := item.value
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(projectDir, candidate)
		}
		canonical, err := canonicalFilesystemPath(candidate)
		if err != nil {
			return fmt.Errorf("projectbundle: canonicalize %s %q: %w", item.field, item.value, err)
		}
		for i, root := range canonicalRoots {
			if isSameOrWithin(canonical, root) {
				return fmt.Errorf("%w: %s %q is within collected root %q", ErrInvalidPath, item.field, item.value, roots[i])
			}
		}
	}
	return nil
}

func canonicalFilesystemPath(name string) (string, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	current := absolute
	var suffix []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

type observedRoot struct {
	path string
	info os.FileInfo
}

func observeRoot(name string) (observedRoot, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return observedRoot{}, err
	}
	absolute = filepath.Clean(absolute)
	before, err := os.Stat(absolute)
	if err != nil {
		return observedRoot{}, err
	}
	if !before.IsDir() {
		return observedRoot{}, fmt.Errorf("%w: root %q is not a directory", ErrUnsafeFile, name)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return observedRoot{}, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return observedRoot{}, err
	}
	canonical = filepath.Clean(canonical)
	after, err := os.Lstat(canonical)
	if err != nil {
		return observedRoot{}, err
	}
	if after.Mode()&os.ModeSymlink != 0 || !after.IsDir() || !os.SameFile(before, after) {
		return observedRoot{}, fmt.Errorf("%w: root %q changed while it was canonicalized", ErrUnsafeFile, name)
	}
	return observedRoot{path: canonical, info: before}, nil
}

func isSameOrWithin(name, root string) bool {
	relative, err := filepath.Rel(root, name)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

type archiveLimitWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *archiveLimitWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.remaining {
		return 0, fmt.Errorf("%w: ZIP exceeds archive byte limit", ErrLimit)
	}
	n, err := w.writer.Write(data)
	w.remaining -= int64(n)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}
