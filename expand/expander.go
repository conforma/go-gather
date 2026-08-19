// Copyright The Enterprise Contract Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package expand defines the Expander interface and a registry of expanders for compressed files.
package expand

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// Expander defines the interface for extracting compressed archives.
type Expander interface {
	Expand(ctx context.Context, source string, destination string, umask os.FileMode) error
	Matcher(extension string) bool
}

var expanders []Expander

// Default resource limits applied to expanders to guard against decompression
// bombs. DefaultMaxEntrySize matches the Conforma CLI's maxTarEntrySize (500 MiB
// per extracted entry); DefaultMaxTotalSize caps the whole-archive extracted
// size; DefaultFilesLimit caps the entry count.
const (
	DefaultMaxEntrySize int64 = 500 * 1024 * 1024
	DefaultMaxTotalSize int64 = 1024 * 1024 * 1024
	DefaultFilesLimit   int   = 10000
)

// ExpandOptions holds extraction limits. For every limit the zero value means
// "use the default" (so a zero-value expander is safe, not unlimited), and a
// negative value explicitly disables that check.
type ExpandOptions struct {
	// MaxEntrySize bounds the size of a single extracted entry.
	MaxEntrySize int64
	// MaxTotalSize bounds the cumulative extracted size across the archive.
	MaxTotalSize int64
	// FilesLimit bounds the number of entries extracted.
	FilesLimit int
}

// Option overrides a field of ExpandOptions.
type Option func(*ExpandOptions)

// WithMaxEntrySize sets the maximum size of a single extracted entry. Zero uses
// the default; a negative value disables the check.
func WithMaxEntrySize(n int64) Option {
	return func(o *ExpandOptions) { o.MaxEntrySize = n }
}

// WithMaxTotalSize sets the maximum cumulative extracted size. Zero uses the
// default; a negative value disables the check.
func WithMaxTotalSize(n int64) Option {
	return func(o *ExpandOptions) { o.MaxTotalSize = n }
}

// WithFilesLimit sets the maximum number of entries extracted. Zero uses the
// default; a negative value disables the check.
func WithFilesLimit(n int) Option {
	return func(o *ExpandOptions) { o.FilesLimit = n }
}

// DefaultExpandOptions returns the safe default limits.
func DefaultExpandOptions() ExpandOptions {
	return ExpandOptions{
		MaxEntrySize: DefaultMaxEntrySize,
		MaxTotalSize: DefaultMaxTotalSize,
		FilesLimit:   DefaultFilesLimit,
	}
}

// ResolveOptions returns ExpandOptions starting from the safe defaults with the
// given overrides applied.
func ResolveOptions(opts ...Option) ExpandOptions {
	o := DefaultExpandOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// Effective resolves the options to the limits an expander enforces: zero values
// use the defaults and negative values disable the corresponding check. Expanders
// embed ExpandOptions and call this so the resolution lives in one place.
func (o ExpandOptions) Effective() (maxEntry, maxTotal int64, filesLimit int) {
	return effectiveSize(o.MaxEntrySize, DefaultMaxEntrySize),
		effectiveSize(o.MaxTotalSize, DefaultMaxTotalSize),
		effectiveCount(o.FilesLimit, DefaultFilesLimit)
}

// effectiveSize resolves a configured size limit: zero uses def, a negative value
// disables the check (math.MaxInt64), and a positive value is used as-is. This
// makes a zero-value expander safe while still allowing an explicit opt-out.
func effectiveSize(v, def int64) int64 {
	switch {
	case v < 0:
		return math.MaxInt64
	case v == 0:
		return def
	default:
		return v
	}
}

// effectiveCount is effectiveSize for entry counts.
func effectiveCount(v, def int) int {
	switch {
	case v < 0:
		return math.MaxInt
	case v == 0:
		return def
	default:
		return v
	}
}

// limitWriter wraps an io.Writer, adding each write's length to *copied and
// failing once *copied exceeds max. It lets a single running budget be enforced
// across multiple io.Copy operations (e.g. an archive-wide total).
type limitWriter struct {
	dst    io.Writer
	copied *int64
	max    int64
}

func (w *limitWriter) Write(p []byte) (int, error) {
	*w.copied += int64(len(p))
	if *w.copied > w.max {
		return 0, fmt.Errorf("extracted data exceeds size limit of %d bytes", w.max)
	}
	return w.dst.Write(p)
}

// CopyBounded copies src into dst using buf as scratch space, adding the bytes
// actually written to *copied and failing if *copied exceeds max. Enforcing on
// bytes actually read defeats archives that under-declare their sizes. Callers
// reuse one buf across many copies to avoid a per-call allocation.
func CopyBounded(dst io.Writer, src io.Reader, copied *int64, max int64, buf []byte) error {
	_, err := io.CopyBuffer(&limitWriter{dst: dst, copied: copied, max: max}, src, buf)
	return err
}

// RemoveOnError removes path when *err is non-nil and created is true. Use it as
// `defer expand.RemoveOnError(dst, createdDst, &err)` so a failed extraction does
// not leave partial (possibly attacker-controlled) output behind, without
// deleting a destination that already existed.
func RemoveOnError(path string, created bool, err *error) {
	if *err != nil && created {
		_ = os.RemoveAll(path)
	}
}

// GetExpander returns the first registered Expander whose Matcher accepts the given extension.
func GetExpander(extension string) Expander {
	for _, expander := range expanders {
		if expander.Matcher(extension) {
			return expander
		}
	}
	return nil
}

// RegisterExpander adds an Expander to the global registry.
func RegisterExpander(e Expander) {
	expanders = append(expanders, e)
}

// Known magic numbers for common compressed file formats
var magicNumbers = map[string][]byte{
	"gzip":  {0x1f, 0x8b},
	"zip":   {0x50, 0x4b, 0x03, 0x04},
	"bzip2": {0x42, 0x5a, 0x68},
	"xz":    {0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00},
	"7z":    {0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c},
}

// IsCompressedFile reports whether the file at filePath begins with a known compression magic number.
func IsCompressedFile(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	// Read the first few bytes
	header := make([]byte, 10) // maximum length of magic numbers
	_, err = file.Read(header)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			return false, fmt.Errorf("could not read file header: %w", err)
		}
		return false, nil
	}

	// Check against known magic numbers
	for _, magic := range magicNumbers {
		if len(header) >= len(magic) && bytes.Equal(header[:len(magic)], magic) {
			return true, nil
		}
	}

	return false, nil
}

// IsTarFile checks whether the file at filePath is a tar archive by reading
// the standard tar magic bytes at offset 257 ("ustar\0" or "ustar ").
func IsTarFile(filePath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	// Move to the position where the tar magic string should appear.
	_, err = f.Seek(257, io.SeekStart)
	if err != nil {
		return false, fmt.Errorf("could not seek in file: %w", err)
	}

	// Read the 6 bytes of the magic string ("ustar\0" or "ustar ").
	magic := make([]byte, 6)
	n, err := f.Read(magic)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("could not read magic bytes: %w", err)
	}

	// If we didn't get enough bytes, it can't be a valid tar.
	if n < 6 {
		return false, nil
	}

	// Check if we have "ustar" at the start (POSIX tar magic).
	if bytes.HasPrefix(magic, []byte("ustar")) {
		return true, nil
	}

	return false, nil
}
