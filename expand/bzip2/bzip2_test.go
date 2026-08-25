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

package bzip2

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/conforma/go-gather/expand"
	"github.com/conforma/go-gather/internal/helpers"
)

func TestNewBzip2Expander_Defaults(t *testing.T) {
	e := NewBzip2Expander()
	if e.MaxEntrySize != expand.DefaultMaxEntrySize {
		t.Errorf("MaxEntrySize = %d, want %d", e.MaxEntrySize, expand.DefaultMaxEntrySize)
	}
}

func TestNewBzip2Expander_Override(t *testing.T) {
	e := NewBzip2Expander(expand.WithMaxEntrySize(99))
	if e.MaxEntrySize != 99 {
		t.Errorf("MaxEntrySize = %d, want 99", e.MaxEntrySize)
	}
}

// TestBzip2Expander_Expand_AtSizeLimit verifies that a file whose size is exactly
// the limit is accepted (the over-limit rejection is covered in
// TestBzip2Expander_Expand).
func TestBzip2Expander_Expand_AtSizeLimit(t *testing.T) {
	// The fixture decompresses to "Hello Bzip2!" (12 bytes).
	e := NewBzip2Expander(expand.WithMaxEntrySize(12))
	bz2Path := createBzip2Fixture(t)
	dstDir := t.TempDir()
	if err := e.Expand(context.Background(), bz2Path, dstDir, 0o755); err != nil {
		t.Fatalf("extraction at exactly the limit should succeed, got %v", err)
	}
}

// TestBzip2Expander_Expand_PreservesExistingOnFailure verifies that a failed
// (over-limit) decode leaves a pre-existing output file untouched, thanks to the
// atomic temp-file-then-rename write.
func TestBzip2Expander_Expand_PreservesExistingOnFailure(t *testing.T) {
	bz2Path := createBzip2Fixture(t) // decompresses to "Hello Bzip2!" (12 bytes)
	dstDir := t.TempDir()
	// The output path is dst/<base without .bz2>; the fixture is "test.txt.bz2".
	outPath := filepath.Join(dstDir, "test.txt")
	const original = "ORIGINAL"
	if err := os.WriteFile(outPath, []byte(original), 0600); err != nil {
		t.Fatalf("failed to seed existing output: %v", err)
	}

	e := NewBzip2Expander(expand.WithMaxEntrySize(3)) // 12 > 3 → fails
	if err := e.Expand(context.Background(), bz2Path, dstDir, 0o755); err == nil {
		t.Fatalf("expected over-limit failure, got nil")
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("existing output should still exist: %v", err)
	}
	if string(got) != original {
		t.Errorf("existing output was clobbered on failure: got %q, want %q", got, original)
	}
}

// helloBzip2Fixture is a small bzip2-encoded byte slice that decompresses to "Hello Bzip2!".
// This was generated externally to ensure its validity.
var helloBzip2Fixture = []byte{
	0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0x8e, 0x9d,
	0x35, 0x69, 0x00, 0x00, 0x02, 0x1d, 0x80, 0x60, 0x00, 0x10, 0x00, 0x10,
	0x40, 0x02, 0x24, 0xc0, 0x10, 0x20, 0x00, 0x31, 0x00, 0xd3, 0x4d, 0x04,
	0x0d, 0x06, 0x9a, 0x11, 0xc2, 0xb1, 0x14, 0xc9, 0x78, 0xbb, 0x92, 0x29,
	0xc2, 0x84, 0x84, 0x74, 0xe9, 0xab, 0x48,
}

// TestBzip2Expander_Matcher tests the Matcher function for various file extensions.
func TestBzip2Expander_Matcher(t *testing.T) {
	expander := &Bzip2Expander{}

	tests := []struct {
		name      string
		extension string
		want      bool
	}{
		{"bz2 simple", "file.bz2", true},
		{"bzip2 substring", "archive.bzip2", true},
		{"tar.bz2 false", "archive.tar.bz2", false},
		{"zip false", "file.zip", false},
		{"bzip2-tar substring false", "something-bzip2.tar", false},
		{"bzip2 random substring true", "something-bzip2", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expander.Matcher(tc.extension)
			if got != tc.want {
				t.Errorf("Matcher(%q) = %v, want %v", tc.extension, got, tc.want)
			}
		})
	}
}

// TestBzip2Expander_Expand contains all tests for the Expand method.
func TestBzip2Expander_Expand(t *testing.T) {
	expander := NewBzip2Expander(expand.WithMaxEntrySize(1024)) // 1 KB limit

	// Positive Test: Successfully decompresses a valid bzip2 file into a directory.
	t.Run("positive: decompresses valid bzip2 file into directory", func(t *testing.T) {
		ctx := context.Background()

		bz2Path := createBzip2Fixture(t)
		dstDir := t.TempDir()

		err := expander.Expand(ctx, bz2Path, dstDir, 0o755)
		if err != nil {
			t.Fatalf("Expand returned error, want=nil got=%v", err)
		}

		expectedOutputFileName := strings.TrimSuffix(filepath.Base(bz2Path), filepath.Ext(bz2Path))
		outFile := filepath.Join(dstDir, expectedOutputFileName)

		info, err := os.Stat(outFile)
		if err != nil {
			t.Fatalf("decompressed file does not exist: %v", err)
		}
		if info.IsDir() {
			t.Fatalf("decompressed path is a directory, expected a file")
		}

		decompressed, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("failed to read decompressed file: %v", err)
		}
		want := []byte("Hello Bzip2!")
		if !bytes.Equal(decompressed, want) {
			t.Errorf("decompressed content mismatch, want=%q got=%q", string(want), string(decompressed))
		}
	})

	// Negative Test: pathExpanderFunc fails for source path
	t.Run("negative: pathExpanderFunc fails for source path", func(t *testing.T) {
		ctx := context.Background()

		// Mock pathExpanderFunc to fail for source path
		originalPathExpanderFunc := helpers.PathExpanderFunc
		defer func() { pathExpanderFunc = originalPathExpanderFunc }()
		pathExpanderFunc = func(path string) (string, error) {
			if path == "~invalid_src" {
				return "", fmt.Errorf("mocked path expansion failure for source")
			}
			return originalPathExpanderFunc(path)
		}

		invalidSrc := "~invalid_src"
		dstDir := t.TempDir()

		err := expander.Expand(ctx, invalidSrc, dstDir, 0o755)
		if err == nil {
			t.Fatal("expected Expand to fail due to pathExpanderFunc error for source, got nil")
		}
		if !strings.Contains(err.Error(), "failed to expand source path") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// Negative Test: pathExpanderFunc fails for destination path
	t.Run("negative: pathExpanderFunc fails for destination path", func(t *testing.T) {
		ctx := context.Background()
		dst := t.TempDir()

		// Mock pathExpanderFunc to fail for destination path
		originalPathExpanderFunc := helpers.PathExpanderFunc
		defer func() { pathExpanderFunc = originalPathExpanderFunc }()
		pathExpanderFunc = func(path string) (string, error) {
			if path == filepath.Join(dst, "invalid_dst") {
				return "", fmt.Errorf("failed to expand destination path")
			}
			return originalPathExpanderFunc(path)
		}

		bz2Path := createBzip2Fixture(t)
		invalidDst := "invalid_dst"

		err := expander.Expand(ctx, bz2Path, filepath.Join(dst, invalidDst), 0o755)
		if err == nil {
			t.Fatal("expected Expand to fail due to pathExpanderFunc error for destination, got nil")
		}
		if !strings.Contains(err.Error(), "failed to expand destination path") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// Negative Test: Source file does not exist
	t.Run("negative: source file does not exist", func(t *testing.T) {
		ctx := context.Background()

		nonExistentSrc := filepath.Join(t.TempDir(), "nonexistent.bz2")
		dstDir := t.TempDir()

		err := expander.Expand(ctx, nonExistentSrc, dstDir, 0o755)
		if err == nil {
			t.Fatal("expected Expand to fail due to non-existent source file, got nil")
		}
		if !strings.Contains(err.Error(), "failed to open bzip2 file") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// Negative Test: Fails to create output directory due to permissions
	t.Run("negative: fails to create output directory due to permissions", func(t *testing.T) {
		ctx := context.Background()

		bz2Path := createBzip2Fixture(t)

		readOnlyDir := filepath.Join(t.TempDir(), "readonly_dir")
		if err := os.Mkdir(readOnlyDir, 0o555); err != nil {
			t.Fatalf("failed to create read-only directory: %v", err)
		}

		err := expander.Expand(ctx, bz2Path, readOnlyDir, 0o755)
		if err == nil {
			t.Fatal("expected Expand to fail due to inability to create files in read-only directory, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create file") && !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// Negative Test: Decompressed file exceeds size limit
	t.Run("negative: decompressed file exceeds size limit", func(t *testing.T) {
		ctx := context.Background()

		// Create an expander with a small size limit
		smallExpander := NewBzip2Expander(expand.WithMaxEntrySize(5)) // 5 bytes

		bz2Path := createBzip2Fixture(t)
		dstDir := t.TempDir()

		err := smallExpander.Expand(ctx, bz2Path, dstDir, 0o755)
		if err == nil {
			t.Fatal("expected Expand to fail due to size limit exceeded, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds size limit") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	// Negative Test: Corrupt bzip2 data
	t.Run("negative: corrupt bzip2 data", func(t *testing.T) {
		ctx := context.Background()

		// Create a corrupt bzip2 fixture
		tmpDir := t.TempDir()
		corruptBZ2Path := filepath.Join(tmpDir, "corrupt.bz2")
		if err := os.WriteFile(corruptBZ2Path, []byte("Not a valid bzip2 data"), 0600); err != nil {
			t.Fatalf("failed to write corrupt .bz2 fixture: %v", err)
		}

		dstDir := t.TempDir()

		err := expander.Expand(ctx, corruptBZ2Path, dstDir, 0o755)
		if err == nil {
			t.Fatal("expected Expand to fail due to corrupt bzip2 data, got nil")
		}
		if !strings.Contains(err.Error(), "failed to decompress") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

// createBzip2Fixture writes the embedded bzip2 data to a temporary file and returns its path.
func createBzip2Fixture(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	bz2Path := filepath.Join(tmpDir, "test.txt.bz2")
	if err := os.WriteFile(bz2Path, helloBzip2Fixture, 0600); err != nil {
		t.Fatalf("failed to write .bz2 fixture: %v", err)
	}
	return bz2Path
}
