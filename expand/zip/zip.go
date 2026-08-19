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

// Package zip implements an Expander for ZIP archives.
package zip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/safearchive/zip"

	"github.com/conforma/go-gather/expand"
	"github.com/conforma/go-gather/internal/helpers"
)

var pathExpanderFunc = helpers.ExpandPath

// ZipExpander provides functionality to extract ZIP archives. Its limits come
// from the embedded ExpandOptions: each zero value uses the default and a
// negative value disables the check, so a zero-value ZipExpander is safe.
type ZipExpander struct {
	expand.ExpandOptions
}

// NewZipExpander returns a ZipExpander with safe default resource limits,
// overridable via options.
func NewZipExpander(opts ...expand.Option) *ZipExpander {
	return &ZipExpander{ExpandOptions: expand.ResolveOptions(opts...)}
}

// Expand extracts a ZIP file to the specified destination directory.
// It handles tilde expansion, enforces file size limits, and ensures secure extraction.
func (z *ZipExpander) Expand(ctx context.Context, src, dst string, umask os.FileMode) (err error) {
	src, err = pathExpanderFunc(src)
	if err != nil {
		return fmt.Errorf("failed to expand source path: %w", err)
	}

	dst, err = pathExpanderFunc(dst)
	if err != nil {
		return fmt.Errorf("failed to expand destination path: %w", err)
	}

	// Open the ZIP archive
	archive, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("failed to open zip file %q: %w", src, err)
	}
	defer archive.Close()

	maxEntry, maxTotal, filesLimit := z.Effective()

	// Remove partial output on failure so a rejected archive does not leave
	// extracted data behind, but only if we created dst (never delete a
	// pre-existing destination).
	_, dstStatErr := os.Stat(dst)
	createdDst := os.IsNotExist(dstStatErr)
	defer expand.RemoveOnError(dst, createdDst, &err)

	// Enforce the entry-count limit up front to reject many-file bombs before
	// extracting anything. The central directory gives the full count.
	if len(archive.File) > filesLimit {
		return fmt.Errorf("zip archive contains %d files, exceeding the limit of %d", len(archive.File), filesLimit)
	}

	// totalExtracted tracks the cumulative extracted size across all entries so
	// many individually-valid entries cannot exceed the total in aggregate. One
	// buffer is reused across entries to avoid a per-entry allocation.
	var totalExtracted int64
	buf := make([]byte, 32*1024)

	// Iterate over files in the archive
	for _, f := range archive.File {
		// Enforce the per-entry declared size limit as a cheap early reject.
		if f.FileInfo().Size() > maxEntry {
			return fmt.Errorf("zip entry %q size %d exceeds the per-entry limit of %d bytes", f.Name, f.FileInfo().Size(), maxEntry)
		}

		// Construct full file path. safearchive prevents Zip Slip.
		filePath := filepath.Join(dst, f.Name) // nolint:gosec

		if !helpers.IsPathContained(filePath, dst) {
			return fmt.Errorf("illegal file path: %s", filePath)
		}

		// Handle directories
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(filePath, umask); err != nil {
				return fmt.Errorf("failed to create directory %q: %w", filePath, err)
			}
			continue
		}

		// Ensure destination directory exists
		if err = os.MkdirAll(filepath.Dir(filePath), umask); err != nil {
			return fmt.Errorf("failed to create directory %q: %w", filepath.Dir(filePath), err)
		}

		// Extract the file
		if err = z.extractFile(f, filePath, &totalExtracted, maxTotal, buf); err != nil {
			return err
		}
	}

	return nil
}

// extractFile extracts a single file from the ZIP archive, enforcing the
// archive-wide total size limit on the actual bytes written via
// expand.CopyBounded. totalExtracted accumulates across the whole archive.
func (z *ZipExpander) extractFile(f *zip.File, filePath string, totalExtracted *int64, maxTotal int64, buf []byte) (err error) {
	srcFile, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", f.Name, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", filePath, err)
	}
	defer func() {
		if cerr := dstFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if err := expand.CopyBounded(dstFile, srcFile, totalExtracted, maxTotal, buf); err != nil {
		return fmt.Errorf("failed to extract file %q: %w", f.Name, err)
	}
	return nil
}

// Matcher checks if the extension matches supported formats.
func (z *ZipExpander) Matcher(extension string) bool {
	return strings.Contains(extension, "zip")
}

func init() {
	expand.RegisterExpander(NewZipExpander())
}
