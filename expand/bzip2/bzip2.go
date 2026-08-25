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

// Package bzip2 implements an Expander for standalone bzip2 compressed files.
package bzip2

import (
	"compress/bzip2"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/conforma/go-gather/expand"
	"github.com/conforma/go-gather/internal/helpers"
)

var pathExpanderFunc = helpers.ExpandPath

// Bzip2Expander decompresses standalone bzip2 files (not tar.bz2). Its size
// limit comes from the embedded ExpandOptions (bzip2 is a single stream, so only
// MaxEntrySize applies): zero uses the default and a negative value disables the
// check, so a zero-value Bzip2Expander is safe rather than unlimited.
type Bzip2Expander struct {
	expand.ExpandOptions
}

// NewBzip2Expander returns a Bzip2Expander with a safe default size limit,
// overridable via options.
func NewBzip2Expander(opts ...expand.Option) *Bzip2Expander {
	return &Bzip2Expander{ExpandOptions: expand.ResolveOptions(opts...)}
}

// Expand decompresses a bzip2 file from src into the dst directory.
func (b *Bzip2Expander) Expand(ctx context.Context, src, dst string, umask os.FileMode) (err error) {
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
		return fmt.Errorf("failed to open bzip2 file %q: %w", src, err)
	}
	defer input.Close()

	bzipReader := bzip2.NewReader(input)

	// Ensure the parent directory of dst exists
	if err := os.MkdirAll(dst, umask); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", filepath.Dir(dst), err)
	}

	baseName := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))

	fpath := filepath.Join(dst, baseName)

	// Decode into a sibling temporary file and rename it into place only after a
	// successful, size-validated decode. This is a single-file output (like the
	// HTTP gatherer), so an atomic replace means a rejected bomb or failed decode
	// never truncates or destroys a pre-existing output.
	tmp, err := os.CreateTemp(dst, ".gather-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %q: %w", dst, err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	// bzip2 is a single decompressed stream, so bound it by the smaller of the
	// per-entry and total limits (neither embedded option is silently ignored);
	// FilesLimit is not applicable. Zero uses the default and a negative value
	// disables the check, so a zero-value expander stays safe.
	maxEntry, maxTotal, _ := b.Effective()
	limit := min(maxEntry, maxTotal)

	buf := make([]byte, 32*1024)
	if err = expand.CopyBounded(tmp, bzipReader, new(int64), limit, buf); err != nil {
		return fmt.Errorf("failed to decompress bzip2 file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err = os.Chmod(tmpName, 0o644); err != nil { //#nosec G302 -- preserve the prior bzip2 output mode (0644)
		return fmt.Errorf("failed to set output file mode: %w", err)
	}
	if err = os.Rename(tmpName, fpath); err != nil {
		return fmt.Errorf("failed to move decompressed file into place: %w", err)
	}
	committed = true

	return nil
}

// Matcher checks if the extension matches supported formats.
func (b *Bzip2Expander) Matcher(extension string) bool {
	return (strings.Contains(extension, "bz2") || strings.Contains(extension, "bzip2")) && !strings.Contains(extension, "tar")
}

func init() {
	expand.RegisterExpander(NewBzip2Expander())
}
