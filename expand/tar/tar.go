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

// Package tar implements an Expander for tar archives, including gzip and bzip2 compressed variants.
package tar

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/safearchive/tar"

	"github.com/conforma/go-gather/expand"
	"github.com/conforma/go-gather/internal/helpers"
)

var (
	pathExpanderFunc = helpers.ExpandPath
	extractTarGzFunc = extractTarGz
	extractTarBzFunc = extractTarBz
	untarFunc        = untar
)

// TarExpander extracts tar, tar.gz, and tar.bz2 archives. Its limits come from
// the embedded ExpandOptions: each zero value uses the default and a negative
// value disables the check, so a zero-value TarExpander is safe, not unlimited.
type TarExpander struct {
	expand.ExpandOptions
}

// NewTarExpander returns a TarExpander with safe default resource limits,
// overridable via options.
func NewTarExpander(opts ...expand.Option) *TarExpander {
	return &TarExpander{ExpandOptions: expand.ResolveOptions(opts...)}
}

// Expand extracts a tar archive from src into the dst directory.
func (t *TarExpander) Expand(ctx context.Context, src, dst string, umask os.FileMode) (err error) {

	src, err = pathExpanderFunc(src)
	if err != nil {
		return fmt.Errorf("failed to expand source path: %w", err)
	}
	dst, err = pathExpanderFunc(dst)
	if err != nil {
		return fmt.Errorf("failed to expand destination path: %w", err)
	}

	input, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %s", src)
	}
	defer input.Close()

	maxEntry, maxTotal, filesLimit := t.Effective()

	// Remove partial output on failure so a rejected archive does not leave
	// extracted (possibly attacker-controlled) data behind, but only if we
	// created dst (never delete a pre-existing destination).
	_, dstStatErr := os.Stat(dst)
	createdDst := os.IsNotExist(dstStatErr)
	defer expand.RemoveOnError(dst, createdDst, &err)

	switch {
	case strings.Contains(src, "tar.gz") || strings.Contains(src, "tgz"):
		err = extractTarGzFunc(input, dst, maxEntry, maxTotal, filesLimit)
	case strings.Contains(src, "tar.bz2") || strings.Contains(src, "tbz2"):
		err = extractTarBzFunc(input, dst, src, maxEntry, maxTotal, filesLimit)
	default:
		err = untarFunc(input, dst, src, maxEntry, maxTotal, filesLimit)
	}

	if err != nil {
		return fmt.Errorf("failed to extract tar archive: %w", err)
	}

	return nil
}

// Matcher returns true if the file name contains a tar-related extension.
func (t *TarExpander) Matcher(fileName string) bool {
	extensions := []string{"tar", "tgz", "tbz2"}
	for _, ext := range extensions {
		if strings.Contains(fileName, ext) {
			return true
		}
	}
	return false
}

// extractTarBz is a helper function that extracts a tarball compressed with bzip2 to a destination directory
func extractTarBz(input io.Reader, dst, src string, maxEntrySize, maxTotalSize int64, filesLimit int) error {
	bzr := bzip2.NewReader(input)
	return untar(bzr, dst, src, maxEntrySize, maxTotalSize, filesLimit)
}

// extractTarGz is a helper function that extracts a tarball compressed with gzip to a destination directory
func extractTarGz(input io.Reader, dst string, maxEntrySize, maxTotalSize int64, filesLimit int) error {
	gzr, err := gzip.NewReader(input)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %s", err)
	}
	defer gzr.Close()

	return untar(gzr, dst, "", maxEntrySize, maxTotalSize, filesLimit)
}

// writeTarFile creates fPath and copies from r into it, enforcing the
// archive-wide total size limit on actual bytes via expand.CopyBounded.
// totalExtracted accumulates across entries and buf is reused across calls.
func writeTarFile(fPath string, mode os.FileMode, r io.Reader, totalExtracted *int64, maxTotalSize int64, buf []byte) (err error) {
	outFile, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("error creating file (%s): %w", fPath, err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	if err = expand.CopyBounded(outFile, r, totalExtracted, maxTotalSize, buf); err != nil {
		return fmt.Errorf("error extracting file (%s): %w", fPath, err)
	}
	return nil
}

// untar is a helper function that untars a tarball to a destination directory
// based on the provided limits. The limits are already resolved to effective
// values (see ExpandOptions.Effective), so they are enforced unconditionally.
func untar(input io.Reader, dst, src string, maxEntrySize, maxTotalSize int64, filesLimit int) error {
	tarReader := tar.NewReader(input)

	seenDirs := map[string]*tar.Header{}
	now := time.Now()

	var (
		totalExtracted int64 // actual bytes written, enforced against maxTotalSize
		filesCount     int
	)
	buf := make([]byte, 32*1024) // reused across entries

	// Initialize a counter for headers processed
	headerCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			if headerCount == 0 {
				return fmt.Errorf("tar file is empty: %s", src)
			}
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar header: %w", err)
		}

		headerCount++

		// Skip extended headers before counting so they don't count as files.
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}

		// Validate the file count limit
		filesCount++
		if filesCount > filesLimit {
			return fmt.Errorf("tar file contains more files than the %d allowed: %d", filesLimit, filesCount)
		}

		// Construct the file path safely to prevent Zip Slip
		fPath := filepath.Join(dst, header.Name) // #nosec G305 we're checking the path below
		if !helpers.IsPathContained(fPath, dst) {
			return fmt.Errorf("illegal file path: %s", fPath)
		}

		fileInfo := header.FileInfo()
		if !fileInfo.IsDir() {
			// Per-entry declared-size early reject. The archive-wide total is
			// enforced on actual bytes during the copy (see writeTarFile), which
			// also catches entries that under-declare their size in the header.
			if fileInfo.Size() > maxEntrySize {
				return fmt.Errorf("tar entry %q size %d exceeds the per-entry limit of %d bytes", header.Name, fileInfo.Size(), maxEntrySize)
			}
		}

		if fileInfo.IsDir() {
			// Create directories and store their headers for later permission/timestamp adjustment
			if err := os.MkdirAll(fPath, 0755); err != nil { // Use a reasonable default, e.g., 0755
				return fmt.Errorf("failed to create directory (%s): %w", fPath, err)
			}
			seenDirs[fPath] = header
			continue
		}

		// Ensure the parent directory exists
		destPath := filepath.Dir(fPath)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := os.MkdirAll(destPath, 0755); err != nil { // Use a reasonable default
				return fmt.Errorf("failed to create directory (%s): %w", destPath, err)
			}
		}
		// Extract the file

		if err := writeTarFile(fPath, header.FileInfo().Mode(), tarReader, &totalExtracted, maxTotalSize, buf); err != nil {
			return err
		}

		// Set file times
		aTime, mTime := now, now
		if !header.AccessTime.IsZero() {
			aTime = header.AccessTime
		}
		if !header.ModTime.IsZero() {
			mTime = header.ModTime
		}
		if err := os.Chtimes(fPath, aTime, mTime); err != nil {
			return fmt.Errorf("failed to change file times (%s): %w", fPath, err)
		}
	}

	// Adjust directory permissions and timestamps
	for path, dirHeader := range seenDirs {
		// Set permissions
		if err := os.Chmod(path, dirHeader.FileInfo().Mode()); err != nil {
			return fmt.Errorf("failed to change directory permissions (%s): %w", path, err)
		}

		// Set timestamps
		aTime, mTime := now, now
		if !dirHeader.AccessTime.IsZero() {
			aTime = dirHeader.AccessTime
		}
		if !dirHeader.ModTime.IsZero() {
			mTime = dirHeader.ModTime
		}
		if err := os.Chtimes(path, aTime, mTime); err != nil {
			return fmt.Errorf("failed to change directory times (%s): %w", path, err)
		}
	}

	return nil
}

func init() {
	expand.RegisterExpander(NewTarExpander())
}
