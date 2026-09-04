/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package runtimebundle

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"
)

// VerifyOptions controls verification of an untrusted ZIP. Expected, when
// non-nil, is compared entry-for-entry (including mode, size and full digest)
// after the archive has been read. A nil Expected causes the archive manifest
// to be derived from its contents.
type VerifyOptions struct {
	Limits   Limits
	Expected *Bundle
	// MaterializeSymlinksAsFiles preserves the legacy buildctl behavior for
	// vetted toolchain archives: a ZIP symlink's validated target text is
	// represented and extracted as an ordinary file. It never creates a
	// filesystem symlink and is disabled by default.
	MaterializeSymlinksAsFiles bool
}

type verifiedArchive struct {
	bundle  Bundle
	entries []verifiedEntry
}

type verifiedEntry struct {
	entry Entry
	file  *zip.File
}

type zipDataRange struct {
	start int64
	end   int64
	name  string
}

// VerifyZip opens and fully verifies a ZIP. It hashes every regular entry, so
// a successful result can safely be used as a content address. The source is
// never extracted by this function.
func VerifyZip(zipPath string, options ...VerifyOptions) (Bundle, error) {
	opts, err := oneVerifyOption("VerifyZip", options)
	if err != nil {
		return Bundle{}, err
	}
	file, err := openSourceZip(zipPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: open ZIP %s: %w", zipPath, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: stat ZIP %s: %w", zipPath, err)
	}
	archive, err := verifyReaderAt(file, info.Size(), opts)
	if err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: verify ZIP %s: %w", zipPath, err)
	}
	return archive.bundle, nil
}

// VerifyZipReader verifies a ZIP held by an io.ReaderAt. size must be the
// exact archive size; supplying a larger or negative size is rejected by the
// archive parser and offset checks.
func VerifyZipReader(reader io.ReaderAt, size int64, options ...VerifyOptions) (Bundle, error) {
	opts, err := oneVerifyOption("VerifyZipReader", options)
	if err != nil {
		return Bundle{}, err
	}
	archive, err := verifyReaderAt(reader, size, opts)
	if err != nil {
		return Bundle{}, err
	}
	return archive.bundle, nil
}

// ExtractZip verifies and extracts an archive into dst. dst must be a private,
// newly-created directory owned by the caller when the input is untrusted.
// This function does not publish dst atomically; Cache.Materialize is the
// higher-level API that performs sibling-temp extraction and atomic rename.
// Symlinked ancestors of dst are allowed as a caller-selected canonical path;
// once dst is opened, os.Root confines archive names below the resolved
// directory and the pathname identity is checked again before success.
func ExtractZip(zipPath, dst string, options ...VerifyOptions) (Bundle, error) {
	opts, err := oneVerifyOption("ExtractZip", options)
	if err != nil {
		return Bundle{}, err
	}
	file, err := openSourceZip(zipPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: open ZIP %s: %w", zipPath, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: stat ZIP %s: %w", zipPath, err)
	}
	archive, err := verifyReaderAt(file, info.Size(), opts)
	if err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: verify ZIP %s: %w", zipPath, err)
	}
	if err := ensureExtractionRoot(dst); err != nil {
		return Bundle{}, err
	}
	if err := extractVerified(archive, dst); err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: extract ZIP %s: %w", zipPath, err)
	}
	return archive.bundle, nil
}

// ExtractZipReader verifies and extracts a ZIP from an already-open reader.
// Keeping the reader open lets callers bind verification and extraction to one
// file object without resolving an ambient path between those operations.
func ExtractZipReader(reader io.ReaderAt, size int64, dst string, options ...VerifyOptions) (Bundle, error) {
	opts, err := oneVerifyOption("ExtractZipReader", options)
	if err != nil {
		return Bundle{}, err
	}
	archive, err := verifyReaderAt(reader, size, opts)
	if err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: verify ZIP reader: %w", err)
	}
	if err := extractVerified(archive, dst); err != nil {
		return Bundle{}, fmt.Errorf("runtimebundle: extract ZIP reader: %w", err)
	}
	return archive.bundle, nil
}

func oneVerifyOption(name string, options []VerifyOptions) (VerifyOptions, error) {
	if len(options) > 1 {
		return VerifyOptions{}, fmt.Errorf("runtimebundle: %s accepts at most one options value", name)
	}
	if len(options) == 1 {
		return options[0], nil
	}
	return VerifyOptions{}, nil
}

func verifyReaderAt(reader io.ReaderAt, size int64, options VerifyOptions) (verifiedArchive, error) {
	if reader == nil || isNilReaderAt(reader) {
		return verifiedArchive{}, fmt.Errorf("%w: nil archive reader", ErrUnsafeArchive)
	}
	limits, err := options.Limits.withDefaults()
	if err != nil {
		return verifiedArchive{}, err
	}
	if size < 0 {
		return verifiedArchive{}, fmt.Errorf("%w: negative archive size", ErrUnsafeArchive)
	}
	if size > limits.MaxArchiveBytes {
		return verifiedArchive{}, fmt.Errorf("%w: archive size %d exceeds limit %d", ErrArchiveLimit, size, limits.MaxArchiveBytes)
	}
	if size > 0 && size < 22 {
		return verifiedArchive{}, fmt.Errorf("%w: archive too small", ErrUnsafeArchive)
	}
	if err := preflightZipArchive(reader, size, limits); err != nil {
		if errors.Is(err, ErrArchiveLimit) {
			return verifiedArchive{}, err
		}
		return verifiedArchive{}, fmt.Errorf("%w: preflight ZIP: %v", ErrUnsafeArchive, err)
	}
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return verifiedArchive{}, fmt.Errorf("%w: parse ZIP: %v", ErrUnsafeArchive, err)
	}
	if len(archive.File) > limits.MaxEntries {
		return verifiedArchive{}, fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, len(archive.File), limits.MaxEntries)
	}

	entries := make([]verifiedEntry, 0, len(archive.File))
	seen := make(map[string]Entry, len(archive.File))
	seenNames := make(map[string]string, len(archive.File))
	var ranges []zipDataRange
	var total int64
	emptyDigest := sha256.Sum256(nil)
	for _, file := range archive.File {
		if err := validateZipOffset(file, size); err != nil {
			return verifiedArchive{}, err
		}
		dataOffset, _ := file.DataOffset()
		if file.CompressedSize64 > 0 {
			end := dataOffset + int64(file.CompressedSize64)
			ranges = append(ranges, zipDataRange{start: dataOffset, end: end, name: file.Name})
		}
		if file.Flags&1 != 0 {
			return verifiedArchive{}, fmt.Errorf("%w: encrypted entry %q", ErrUnsafeArchive, file.Name)
		}
		name, key, isDir, err := normalizeEntryName(file.Name)
		if err != nil {
			return verifiedArchive{}, err
		}
		if previous, ok := seenNames[key]; ok {
			if previous == name {
				return verifiedArchive{}, fmt.Errorf("%w: duplicate entry %q", ErrUnsafeArchive, name)
			}
			return verifiedArchive{}, fmt.Errorf("%w: case-fold/normalization collision between %q and %q", ErrUnsafeArchive, previous, name)
		}

		materializedSymlink := options.MaterializeSymlinksAsFiles && file.Mode()&fs.ModeType == fs.ModeSymlink
		mode, err := safeZipMode(file, isDir, options.MaterializeSymlinksAsFiles)
		if err != nil {
			return verifiedArchive{}, err
		}
		if isDir {
			if file.UncompressedSize64 != 0 {
				return verifiedArchive{}, fmt.Errorf("%w: directory %q has non-zero size", ErrUnsafeArchive, name)
			}
			entry := Entry{
				Name: name, Mode: mode, Size: 0, SHA256: hex.EncodeToString(emptyDigest[:]),
			}
			seen[key] = entry
			seenNames[key] = name
			entries = append(entries, verifiedEntry{entry: entry, file: file})
			continue
		}

		if file.UncompressedSize64 > uint64(limits.MaxEntrySize) {
			return verifiedArchive{}, fmt.Errorf("%w: entry %q size %d exceeds limit %d", ErrArchiveLimit, name, file.UncompressedSize64, limits.MaxEntrySize)
		}
		if file.UncompressedSize64 > uint64(limits.MaxTotalSize) || int64(file.UncompressedSize64) > limits.MaxTotalSize-total {
			return verifiedArchive{}, fmt.Errorf("%w: total size exceeds limit %d", ErrArchiveLimit, limits.MaxTotalSize)
		}
		if err := checkCompressionRatio(file, limits.MaxCompressionRatio); err != nil {
			return verifiedArchive{}, err
		}
		var digest string
		var actualSize int64
		if materializedSymlink {
			digest, actualSize, err = hashMaterializedSymlinkEntry(file, name)
		} else {
			digest, actualSize, err = hashZipEntry(file, limits.MaxEntrySize)
		}
		if err != nil {
			return verifiedArchive{}, fmt.Errorf("%w: entry %q: %w", ErrUnsafeArchive, name, err)
		}
		if actualSize != int64(file.UncompressedSize64) {
			return verifiedArchive{}, fmt.Errorf("%w: entry %q declared size %d, read %d", ErrUnsafeArchive, name, file.UncompressedSize64, actualSize)
		}
		if actualSize > limits.MaxTotalSize-total {
			return verifiedArchive{}, fmt.Errorf("%w: total size exceeds limit %d", ErrArchiveLimit, limits.MaxTotalSize)
		}
		total += actualSize
		entry := Entry{
			Name: name, Mode: mode, Size: actualSize, SHA256: digest,
		}
		seen[key] = entry
		seenNames[key] = name
		entries = append(entries, verifiedEntry{entry: entry, file: file})
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start == ranges[j].start {
			return ranges[i].end < ranges[j].end
		}
		return ranges[i].start < ranges[j].start
	})
	for i := 1; i < len(ranges); i++ {
		previous, current := ranges[i-1], ranges[i]
		if current.start < previous.end {
			return verifiedArchive{}, fmt.Errorf("%w: entries %q and %q overlap compressed data", ErrUnsafeArchive, previous.name, current.name)
		}
	}
	for key, entry := range seen {
		parent := key
		for {
			index := strings.LastIndexByte(parent, '/')
			if index < 0 {
				break
			}
			parent = parent[:index]
			if parentEntry, ok := seen[parent]; ok && !parentEntry.isDir() {
				return verifiedArchive{}, fmt.Errorf("%w: file %q is also a parent of %q", ErrUnsafeArchive, parentEntry.Name, entry.Name)
			}
		}
	}

	bundle := Bundle{Schema: SchemaV1, Entries: make([]Entry, 0, len(entries))}
	for _, item := range entries {
		bundle.Entries = append(bundle.Entries, item.entry)
	}
	// ZIP manifests are independent of cache namespace. Namespace is supplied
	// by the cache caller, not guessed from an archive's bytes.
	bundle, err = bundle.WithDigestWithLimits(limits)
	if err != nil {
		return verifiedArchive{}, err
	}
	if options.Expected != nil {
		want := *options.Expected
		originalDigest := want.Digest
		if want.Schema == "" {
			want.Schema = SchemaV1
		}
		want.Digest = ""
		if want.Namespace != "" {
			bundle.Namespace = want.Namespace
			bundle, err = bundle.WithDigestWithLimits(limits)
			if err != nil {
				return verifiedArchive{}, err
			}
		}
		if err := want.ValidateWithLimits(limits); err != nil {
			return verifiedArchive{}, err
		}
		if err := manifestEntriesEqualWithLimits(bundle, want, limits); err != nil {
			return verifiedArchive{}, err
		}
		if originalDigest != "" && bundle.Digest != originalDigest {
			return verifiedArchive{}, fmt.Errorf("%w: archive %s, expected %s", ErrDigestMismatch, bundle.Digest, originalDigest)
		}
	}
	return verifiedArchive{bundle: bundle, entries: entries}, nil
}

func isNilReaderAt(reader io.ReaderAt) bool {
	value := reflect.ValueOf(reader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func validateZipOffset(file *zip.File, archiveSize int64) error {
	dataOffset, err := file.DataOffset()
	if err != nil {
		return fmt.Errorf("%w: entry %q data offset: %v", ErrUnsafeArchive, file.Name, err)
	}
	if dataOffset < 0 || dataOffset > archiveSize {
		return fmt.Errorf("%w: entry %q data offset %d outside archive", ErrUnsafeArchive, file.Name, dataOffset)
	}
	if file.CompressedSize64 > uint64(archiveSize-dataOffset) {
		return fmt.Errorf("%w: entry %q compressed data exceeds archive", ErrUnsafeArchive, file.Name)
	}
	return nil
}

func checkCompressionRatio(file *zip.File, maxRatio uint64) error {
	compressed := file.CompressedSize64
	uncompressed := file.UncompressedSize64
	if uncompressed == 0 {
		return nil
	}
	if compressed == 0 || maxRatio == 0 || compressed > ^uint64(0)/maxRatio || uncompressed > compressed*maxRatio {
		return fmt.Errorf("%w: entry %q compression ratio exceeds %d:1", ErrArchiveLimit, file.Name, maxRatio)
	}
	return nil
}

func hashZipEntry(file *zip.File, maxSize int64) (digest string, size int64, err error) {
	input, err := file.Open()
	if err != nil {
		return "", 0, err
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	hasher := sha256.New()
	limited := io.LimitReader(input, limitWithOverflow(maxSize))
	size, err = io.Copy(hasher, limited)
	if err != nil {
		return "", size, err
	}
	if size > maxSize {
		return "", size, fmt.Errorf("%w: entry exceeds limit %d", ErrArchiveLimit, maxSize)
	}
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

func limitWithOverflow(max int64) int64 {
	const maxInt64 = int64(^uint64(0) >> 1)
	if max >= maxInt64 {
		return maxInt64
	}
	return max + 1
}

func safeZipMode(file *zip.File, isDir, materializeSymlinksAsFiles bool) (uint32, error) {
	mode := file.Mode()
	typeBits := mode & fs.ModeType
	if mode&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
		return 0, fmt.Errorf("%w: entry %q has special permission bits", ErrUnsupportedArchiveEntry, file.Name)
	}
	if typeBits == fs.ModeSymlink && materializeSymlinksAsFiles {
		if isDir {
			return 0, fmt.Errorf("%w: symlink entry %q uses a directory name", ErrUnsupportedArchiveEntry, file.Name)
		}
		return uint32(mode.Perm()), nil
	}
	if typeBits != 0 && typeBits != fs.ModeDir {
		return 0, fmt.Errorf("%w: entry %q has mode %v", ErrUnsupportedArchiveEntry, file.Name, mode)
	}
	if isDir {
		return uint32(fs.ModeDir) | uint32(mode.Perm()), nil
	}
	if mode&fs.ModeDir != 0 {
		return 0, fmt.Errorf("%w: entry %q directory mode/name mismatch", ErrUnsupportedArchiveEntry, file.Name)
	}
	return uint32(mode.Perm()), nil
}

const maxMaterializedSymlinkBytes int64 = 4 << 10

func hashMaterializedSymlinkEntry(file *zip.File, name string) (digest string, size int64, err error) {
	if file.UncompressedSize64 > uint64(maxMaterializedSymlinkBytes) {
		return "", 0, fmt.Errorf("%w: symlink entry %q exceeds %d bytes", ErrArchiveLimit, name, maxMaterializedSymlinkBytes)
	}
	input, err := file.Open()
	if err != nil {
		return "", 0, err
	}
	data, readErr := io.ReadAll(io.LimitReader(input, limitWithOverflow(maxMaterializedSymlinkBytes)))
	closeErr := input.Close()
	if readErr != nil {
		return "", int64(len(data)), readErr
	}
	if closeErr != nil {
		return "", int64(len(data)), closeErr
	}
	if int64(len(data)) > maxMaterializedSymlinkBytes {
		return "", int64(len(data)), fmt.Errorf("%w: symlink entry %q exceeds %d bytes", ErrArchiveLimit, name, maxMaterializedSymlinkBytes)
	}
	if err := validateMaterializedSymlinkTarget(name, data); err != nil {
		return "", int64(len(data)), err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), int64(len(data)), nil
}

func validateMaterializedSymlinkTarget(name string, data []byte) error {
	if len(data) == 0 || !utf8.Valid(data) {
		return fmt.Errorf("%w: symlink entry %q has an empty or non-UTF-8 target", ErrUnsafeArchive, name)
	}
	target := string(data)
	if strings.IndexByte(target, 0) >= 0 {
		return fmt.Errorf("%w: symlink entry %q target contains NUL", ErrUnsafeArchive, name)
	}
	if strings.ContainsRune(target, '\\') {
		return fmt.Errorf("%w: symlink entry %q target contains backslash", ErrUnsafeArchive, name)
	}
	if path.IsAbs(target) || strings.HasPrefix(target, "//") || isDriveAbsolute(target) {
		return fmt.Errorf("%w: symlink entry %q has an absolute target", ErrUnsafeArchive, name)
	}
	resolved := path.Clean(path.Join(path.Dir(name), target))
	if resolved == ".." || strings.HasPrefix(resolved, "../") || path.IsAbs(resolved) {
		return fmt.Errorf("%w: symlink entry %q target escapes the archive root", ErrUnsafeArchive, name)
	}
	return nil
}

func extractVerified(archive verifiedArchive, dst string) error {
	if err := ensureExtractionRoot(dst); err != nil {
		return err
	}
	root, err := openPinnedRoot(dst)
	if err != nil {
		return fmt.Errorf("%w: open confined extraction root: %v", ErrUnsafeArchive, err)
	}
	defer root.Close()
	if err := extractVerifiedRoot(archive, root); err != nil {
		return err
	}
	return checkPinnedRootPath(dst, root)
}

// extractVerifiedRoot performs all destination operations through an os.Root
// pinned to the extraction directory. It also re-hashes each entry while
// writing it: verification and extraction are separate reads of one archive
// inode, so an in-place source mutation between them is detected before the
// caller can publish the result.
func extractVerifiedRoot(archive verifiedArchive, root *os.Root) error {
	for _, item := range archive.entries {
		name := strings.TrimSuffix(item.entry.Name, "/")
		rel := filepath.FromSlash(name)
		if item.entry.isDir() {
			input, openErr := item.file.Open()
			if openErr != nil {
				return openErr
			}
			digest, size, readErr := hashReader(input, 0)
			closeErr := input.Close()
			if readErr != nil || closeErr != nil || size != item.entry.Size || digest != item.entry.SHA256 {
				return fmt.Errorf("%w: extracted directory %s changed", ErrDigestMismatch, item.entry.Name)
			}
			if err := ensureRootDir(root, rel); err != nil {
				return err
			}
			if err := chmodRootDir(root, rel); err != nil {
				return err
			}
			continue
		}
		if err := ensureRootDir(root, filepath.Dir(rel)); err != nil {
			return err
		}
		output, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateFileMode(item.entry.Mode))
		if err != nil {
			return err
		}
		input, openErr := item.file.Open()
		if openErr != nil {
			_ = output.Close()
			_ = root.Remove(rel)
			return openErr
		}
		hasher := sha256.New()
		copied, copyErr := io.Copy(io.MultiWriter(output, hasher), io.LimitReader(input, limitWithOverflow(item.entry.Size)))
		closeInputErr := input.Close()
		// Compare the bytes actually written even when zip.File.Open reports a
		// CRC/read error. The archive was verified earlier, but the source inode
		// may have been modified in place between those two reads; the manifest
		// digest must remain the publication gate in that case.
		if copied != item.entry.Size {
			copyErr = fmt.Errorf("%w: extracted entry %s size %d, want %d", ErrDigestMismatch, item.entry.Name, copied, item.entry.Size)
		} else if digest := hex.EncodeToString(hasher.Sum(nil)); digest != item.entry.SHA256 {
			copyErr = fmt.Errorf("%w: extracted entry %s digest mismatch", ErrDigestMismatch, item.entry.Name)
		}
		if copyErr == nil && runtimeIsUnix() {
			copyErr = output.Chmod(privateFileMode(item.entry.Mode))
		}
		if copyErr == nil {
			copyErr = output.Sync()
		}
		closeOutputErr := output.Close()
		if copyErr != nil {
			_ = root.Remove(rel)
			return copyErr
		}
		if closeInputErr != nil {
			_ = root.Remove(rel)
			return closeInputErr
		}
		if closeOutputErr != nil {
			_ = root.Remove(rel)
			return closeOutputErr
		}
	}
	return nil
}

func ensureRootDir(root *os.Root, name string) error {
	name = filepath.Clean(name)
	if name == "." || name == "" {
		return nil
	}
	current := ""
	for _, part := range strings.Split(name, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: invalid destination component %q", ErrUnsafeArchive, name)
		}
		if current == "" {
			current = part
		} else {
			current = filepath.Join(current, part)
		}
		info, err := root.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: destination contains symlink %s", ErrUnsafeArchive, current)
			}
			if !info.IsDir() {
				return fmt.Errorf("%w: destination component is not a directory: %s", ErrUnsafeArchive, current)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := mkdirPrivateRootChild(root, current); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err = root.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%w: destination component changed while creating: %s", ErrUnsafeArchive, current)
		}
	}
	return nil
}

func openPinnedRoot(path string) (*os.Root, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("%w: root is not a real directory: %s", ErrUnsafeArchive, path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	after, err := root.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = root.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: root changed while opening: %s", ErrUnsafeArchive, path)
	}
	if err := checkPinnedRootPath(path, root); err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

// checkPinnedRootPath checks both sides of a pinned root. The descriptor keeps
// operations confined to the directory observed by root.Stat("."), while the
// pathname check prevents callers from reporting success for a directory that
// has been renamed away and replaced at the public path.
func checkPinnedRootPath(path string, root *os.Root) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%w: root pathname changed: %v", ErrUnsafeArchive, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%w: root pathname is not a real directory: %s", ErrUnsafeArchive, path)
	}
	pinned, err := root.Stat(".")
	if err != nil {
		return fmt.Errorf("%w: stat pinned root: %v", ErrUnsafeArchive, err)
	}
	if !os.SameFile(info, pinned) {
		return fmt.Errorf("%w: root pathname was replaced: %s", ErrUnsafeArchive, path)
	}
	return nil
}

func chmodRootDir(root *os.Root, name string) error {
	if !runtimeIsUnix() || name == "." || name == "" {
		return nil
	}
	dir, err := root.Open(name)
	if err != nil {
		return err
	}
	defer dir.Close()
	info, err := dir.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: destination component is not a directory: %s", ErrUnsafeArchive, name)
	}
	return dir.Chmod(0o700)
}

func ensureNoSymlinkParents(root, target string) error {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if _, err := os.Lstat(root); err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == ".." || part == "." || part == "" {
			return fmt.Errorf("%w: invalid destination component", ErrUnsafeArchive)
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: destination contains symlink %s", ErrUnsafeArchive, current)
		}
	}
	return nil
}

func privateFileMode(mode uint32) os.FileMode {
	if mode&0o111 != 0 {
		return 0o700
	}
	return 0o600
}

func runtimeIsUnix() bool {
	return runtime.GOOS != "windows"
}

func ensureExtractionRoot(dst string) error {
	if dst == "" {
		return fmt.Errorf("runtimebundle: extraction destination is empty")
	}
	if info, err := os.Lstat(dst); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: extraction destination is not a real directory", ErrUnsafeArchive)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return fmt.Errorf("runtimebundle: create extraction directory: %w", err)
	}
	if err := os.Chmod(dst, 0o700); err != nil && runtimeIsUnix() {
		return fmt.Errorf("runtimebundle: make extraction directory private: %w", err)
	}
	// Ancestors of dst may be canonical symlinks (for example /tmp on some
	// systems). Descendants are handled through the pinned os.Root; the final
	// dst pathname is checked before ExtractZip returns.
	return ensureNoSymlinkParents(dst, dst)
}

// openSourceZip refuses symlinked/non-regular source paths and checks that the
// descriptor still names the object observed by Lstat. This closes the common
// path-swap TOCTOU before ZIP parsing; bytes are then verified and extracted
// from the same open descriptor.
func openSourceZip(zipPath string) (*os.File, error) {
	info, err := os.Lstat(zipPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: ZIP source is not a regular non-symlink file: %s", ErrUnsafeArchive, zipPath)
	}
	file, err := os.Open(zipPath)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !os.SameFile(info, opened) {
		_ = file.Close()
		return nil, fmt.Errorf("%w: ZIP source changed while opening: %s", ErrUnsafeArchive, zipPath)
	}
	return file, nil
}
