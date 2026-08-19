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

package expand

import (
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// mockExpander is a trivial expander that returns true if its "keyword" is in the extension.
type mockExpander struct {
	keyword string
}

func (m *mockExpander) Expand(ctx context.Context, source, destination string, umask os.FileMode) error {
	// no-op
	return nil
}

func (m *mockExpander) Matcher(extension string) bool {
	return bytes.Contains([]byte(extension), []byte(m.keyword))
}

// TestRegisterAndGetExpander ensures we can register and retrieve an expander via GetExpander.
func TestRegisterAndGetExpander(t *testing.T) {
	oldExpanders := expanders
	expanders = nil
	defer func() { expanders = oldExpanders }()

	mockFoo := &mockExpander{keyword: "foo"}
	mockBar := &mockExpander{keyword: "bar"}

	RegisterExpander(mockFoo)
	RegisterExpander(mockBar)

	if got := GetExpander("my.foo"); got != mockFoo {
		t.Errorf("expected mockFoo, got %#v", got)
	}
	if got := GetExpander("some.bar"); got != mockBar {
		t.Errorf("expected mockBar, got %#v", got)
	}

	if got := GetExpander("nope.zip"); got != nil {
		t.Errorf("expected nil, got %#v", got)
	}
}

// TestIsCompressedFile checks that known magic numbers are correctly recognized.
func TestDefaultExpandOptions(t *testing.T) {
	o := DefaultExpandOptions()
	if o.MaxEntrySize != DefaultMaxEntrySize {
		t.Errorf("MaxEntrySize = %d, want %d", o.MaxEntrySize, DefaultMaxEntrySize)
	}
	if o.MaxTotalSize != DefaultMaxTotalSize {
		t.Errorf("MaxTotalSize = %d, want %d", o.MaxTotalSize, DefaultMaxTotalSize)
	}
	if o.FilesLimit != DefaultFilesLimit {
		t.Errorf("FilesLimit = %d, want %d", o.FilesLimit, DefaultFilesLimit)
	}
}

// TestResolveOptions_OverridePreservesOtherDefaults ensures a caller setting one
// limit does not accidentally change the others.
func TestResolveOptions_OverridePreservesOtherDefaults(t *testing.T) {
	o := ResolveOptions(WithMaxEntrySize(123))
	if o.MaxEntrySize != 123 {
		t.Errorf("MaxEntrySize = %d, want 123", o.MaxEntrySize)
	}
	if o.MaxTotalSize != DefaultMaxTotalSize || o.FilesLimit != DefaultFilesLimit {
		t.Errorf("other defaults changed: total=%d files=%d", o.MaxTotalSize, o.FilesLimit)
	}
}

// TestEffectiveHelpers verifies the zero-value-safe resolution: zero uses the
// default, a negative value disables the check, and a positive value is used.
func TestEffectiveHelpers(t *testing.T) {
	if got := effectiveSize(0, DefaultMaxEntrySize); got != DefaultMaxEntrySize {
		t.Errorf("effectiveSize(0) = %d, want default %d", got, DefaultMaxEntrySize)
	}
	if got := effectiveSize(-1, DefaultMaxEntrySize); got != math.MaxInt64 {
		t.Errorf("effectiveSize(-1) = %d, want MaxInt64 (disabled)", got)
	}
	if got := effectiveSize(42, DefaultMaxEntrySize); got != 42 {
		t.Errorf("effectiveSize(42) = %d, want 42", got)
	}
	if got := effectiveCount(0, DefaultFilesLimit); got != DefaultFilesLimit {
		t.Errorf("effectiveCount(0) = %d, want default %d", got, DefaultFilesLimit)
	}
	if got := effectiveCount(-1, DefaultFilesLimit); got != math.MaxInt {
		t.Errorf("effectiveCount(-1) = %d, want MaxInt (disabled)", got)
	}
}

func TestIsCompressedFile(t *testing.T) {
	tests := []struct {
		name           string
		magic          []byte
		wantCompressed bool
	}{
		{
			name:           "gzip magic",
			magic:          []byte{0x1f, 0x8b},
			wantCompressed: true,
		},
		{
			name:           "zip magic",
			magic:          []byte{0x50, 0x4b, 0x03, 0x04},
			wantCompressed: true,
		},
		{
			name:           "bzip2 magic",
			magic:          []byte{0x42, 0x5a, 0x68}, // "BZh"
			wantCompressed: true,
		},
		{
			name:           "xz magic",
			magic:          []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00},
			wantCompressed: true,
		},
		{
			name:           "7z magic",
			magic:          []byte{0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c},
			wantCompressed: true,
		},
		{
			name:           "unknown magic",
			magic:          []byte{0xde, 0xad, 0xbe, 0xef},
			wantCompressed: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-magic-*.bin")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			if _, err := tmpFile.Write(tc.magic); err != nil {
				t.Fatalf("failed to write magic bytes: %v", err)
			}
			if _, err := tmpFile.Seek(0, 0); err != nil {
				t.Fatalf("failed to seek temp file: %v", err)
			}

			got, err := IsCompressedFile(tmpFile.Name())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantCompressed {
				t.Errorf("IsCompressedFile(%q) = %v, want %v", tc.name, got, tc.wantCompressed)
			}
		})
	}
}

// TestIsCompressedFile_NonExistent checks that an error is returned when the file does not exist.
func TestIsCompressedFile_NonExistent(t *testing.T) {
	fakePath := filepath.Join(os.TempDir(), "no_such_file_12345.bin")
	got, err := IsCompressedFile(fakePath)
	if err == nil {
		t.Fatalf("expected error when checking a non-existent file, but got nil")
	}
	if got {
		t.Errorf("expected false for isCompressed, got true")
	}
}

// TestIsCompressedFile_EmptyFile checks if an empty file doesn't match any known magic number.
func TestIsCompressedFile_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-*.bin")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	got, err := IsCompressedFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error for empty file: %v", err)
	}
	if got {
		t.Errorf("expected false for empty file, got true")
	}
}

// TestIsTarFile checks that files with the "ustar" magic at offset 257 are recognized as tar.
func TestIsTarFile(t *testing.T) {
	tests := []struct {
		name       string
		offsetData []byte // data written at offset 257
		wantTar    bool
		expectErr  bool
	}{
		{
			name:       "valid tar magic with null",
			offsetData: []byte("ustar\000"), // "ustar\0"
			wantTar:    true,
		},
		{
			name:       "valid tar magic with space",
			offsetData: []byte("ustar "), // "ustar "
			wantTar:    true,
		},
		{
			name:       "invalid magic",
			offsetData: []byte("xyz123"),
			wantTar:    false,
		},
		{
			name:       "not enough bytes",
			offsetData: []byte("us"), // fewer than 6
			wantTar:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-tar-*")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			// Write 257 bytes of filler to ensure offset 257 is reached
			if _, err := tmpFile.Write(bytes.Repeat([]byte{0xaa}, 257)); err != nil {
				t.Fatalf("failed to write filler bytes: %v", err)
			}
			// Write the test's offsetData at position 257
			if _, err := tmpFile.Write(tc.offsetData); err != nil {
				t.Fatalf("failed to write magic bytes at offset 257: %v", err)
			}

			// Seek back to start for safety (not strictly necessary)
			if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
				t.Fatalf("failed to seek temp file: %v", err)
			}

			gotTar, err := IsTarFile(tmpFile.Name())
			if err != nil {
				t.Fatalf("unexpected error from IsTarFile: %v", err)
			}
			if gotTar != tc.wantTar {
				t.Errorf("IsTarFile(%q) = %v, want %v", tc.name, gotTar, tc.wantTar)
			}
		})
	}
}

// TestIsTarFile_NonExistent checks that a non-existent file returns an error.
func TestIsTarFile_NonExistent(t *testing.T) {
	fakePath := filepath.Join(os.TempDir(), "no_such_file.tar")
	gotTar, err := IsTarFile(fakePath)
	if err == nil {
		t.Fatal("expected an error when checking non-existent file, but got nil")
	}
	if gotTar {
		t.Errorf("expected false for isTarFile, got true")
	}
}

// TestIsTarFile_ShortFile checks that a file with fewer than 257 bytes returns false (no error).
func TestIsTarFile_ShortFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "short-file-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write fewer than 257 bytes
	if _, err := tmpFile.Write([]byte("abcd")); err != nil {
		t.Fatalf("failed to write minimal data: %v", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("failed to seek temp file: %v", err)
	}

	gotTar, err := IsTarFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTar {
		t.Errorf("expected false for short file, got true")
	}
}
