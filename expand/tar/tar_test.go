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

package tar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bzip2 "github.com/dsnet/compress/bzip2"

	"github.com/conforma/go-gather/expand"
)

func TestNewTarExpander_Defaults(t *testing.T) {
	e := NewTarExpander()
	if e.MaxEntrySize != expand.DefaultMaxEntrySize {
		t.Errorf("MaxEntrySize = %d, want %d", e.MaxEntrySize, expand.DefaultMaxEntrySize)
	}
	if e.MaxTotalSize != expand.DefaultMaxTotalSize {
		t.Errorf("MaxTotalSize = %d, want %d", e.MaxTotalSize, expand.DefaultMaxTotalSize)
	}
	if e.FilesLimit != expand.DefaultFilesLimit {
		t.Errorf("FilesLimit = %d, want %d", e.FilesLimit, expand.DefaultFilesLimit)
	}
}

func TestNewTarExpander_Override(t *testing.T) {
	e := NewTarExpander(expand.WithMaxEntrySize(42))
	if e.MaxEntrySize != 42 {
		t.Errorf("MaxEntrySize = %d, want 42", e.MaxEntrySize)
	}
	if e.FilesLimit != expand.DefaultFilesLimit {
		t.Errorf("FilesLimit = %d, want default %d", e.FilesLimit, expand.DefaultFilesLimit)
	}
}

// TestTarExpander_Expand_EntrySizeLimit checks that a single entry exceeding the
// per-entry limit is rejected.
func TestTarExpander_Expand_EntrySizeLimit(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "big.tar")
	dstDir := filepath.Join(tempDir, "output")

	if err := createTarFile(srcFile, "big.txt", string(bytes.Repeat([]byte("A"), 100))); err != nil {
		t.Fatalf("failed to create tar file: %v", err)
	}

	e := NewTarExpander(expand.WithMaxEntrySize(10))
	err := e.Expand(context.Background(), srcFile, dstDir, 0)
	if err == nil {
		t.Fatalf("expected size-limit error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size-limit error, got %v", err)
	}
}

// TestTarExpander_Expand_TotalSizeLimit checks that entries individually within
// the per-entry limit but exceeding the archive-wide total are rejected, and
// that the partial output is removed.
func TestTarExpander_Expand_TotalSizeLimit(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "many.tar")
	dstDir := filepath.Join(tempDir, "output")

	if err := createTarFiles(srcFile, []tarEntry{
		{"a.txt", string(bytes.Repeat([]byte("A"), 5))},
		{"b.txt", string(bytes.Repeat([]byte("B"), 5))},
		{"c.txt", string(bytes.Repeat([]byte("C"), 5))},
	}); err != nil {
		t.Fatalf("failed to create tar file: %v", err)
	}

	// Each entry is 5 bytes (<= per-entry 100), total 15 > total limit 12.
	e := NewTarExpander(expand.WithMaxEntrySize(100), expand.WithMaxTotalSize(12))
	err := e.Expand(context.Background(), srcFile, dstDir, 0)
	if err == nil {
		t.Fatalf("expected total-size error, got nil")
	}
	if _, statErr := os.Stat(dstDir); statErr == nil {
		t.Errorf("partial extraction output should have been removed on failure")
	}
}

// TestTarExpander_Expand_FilesLimit checks that an entry count exceeding the
// limit is rejected.
func TestTarExpander_Expand_FilesLimit(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "many.tar")
	dstDir := filepath.Join(tempDir, "output")

	if err := createTarFiles(srcFile, []tarEntry{
		{"a.txt", "a"}, {"b.txt", "b"}, {"c.txt", "c"},
	}); err != nil {
		t.Fatalf("failed to create tar file: %v", err)
	}

	e := NewTarExpander(expand.WithFilesLimit(2))
	err := e.Expand(context.Background(), srcFile, dstDir, 0)
	if err == nil {
		t.Fatalf("expected files-limit error, got nil")
	}
	if !strings.Contains(err.Error(), "more files") {
		t.Errorf("expected files-limit error, got %v", err)
	}
}

// TestTarExpander_ZeroValueIsSafe verifies a zero-value TarExpander enforces the
// default limits rather than being unlimited.
func TestTarExpander_ZeroValueIsSafe(t *testing.T) {
	e := &TarExpander{} // all limits zero
	maxEntry, maxTotal, filesLimit := e.Effective()
	if maxEntry != expand.DefaultMaxEntrySize || maxTotal != expand.DefaultMaxTotalSize || filesLimit != expand.DefaultFilesLimit {
		t.Errorf("zero-value limits = (%d, %d, %d), want defaults (%d, %d, %d)",
			maxEntry, maxTotal, filesLimit,
			expand.DefaultMaxEntrySize, expand.DefaultMaxTotalSize, expand.DefaultFilesLimit)
	}
}

// TestTarExpander_Matcher tests the Matcher method for different file names.
func TestTarExpander_Matcher(t *testing.T) {
	tarExpander := &TarExpander{}

	testCases := []struct {
		name     string
		fileName string
		want     bool
	}{
		{
			name:     "tar file",
			fileName: "archive.tar",
			want:     true,
		},
		{
			name:     "tgz file",
			fileName: "archive.tgz",
			want:     true,
		},
		{
			name:     "tbz2 file",
			fileName: "archive.tbz2",
			want:     true,
		},
		{
			name:     "non-tar extension",
			fileName: "archive.zip",
			want:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tarExpander.Matcher(tc.fileName)
			if got != tc.want {
				t.Errorf("Matcher(%s) = %v, want %v", tc.fileName, got, tc.want)
			}
		})
	}
}

// TestTarExpander_Expand_Tar tests extracting a simple .tar file.
func TestTarExpander_Expand_Tar(t *testing.T) {
	tarExpander := &TarExpander{}

	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "test.tar")
	dstDir := filepath.Join(tempDir, "output")

	err := createTarFile(srcFile, "hello.txt", "Hello, world!")
	if err != nil {
		t.Fatalf("failed to create tar file: %v", err)
	}

	ctx := context.Background()
	err = tarExpander.Expand(ctx, srcFile, dstDir, 0)
	if err != nil {
		t.Fatalf("Expand returned an unexpected error: %v", err)
	}

	extractedFile := filepath.Join(dstDir, "hello.txt")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist, but it does not", extractedFile)
	}
}

// TestTarExpander_Expand_TarGz tests extracting a simple .tar.gz file.
func TestTarExpander_Expand_TarGz(t *testing.T) {
	tarExpander := &TarExpander{}

	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "test.tar.gz")
	dstDir := filepath.Join(tempDir, "output")

	err := createTarGzFile(srcFile, "greeting.txt", "Hello from tar.gz!")
	if err != nil {
		t.Fatalf("failed to create tar.gz file: %v", err)
	}

	ctx := context.Background()
	err = tarExpander.Expand(ctx, srcFile, dstDir, 0)
	if err != nil {
		t.Fatalf("Expand returned an unexpected error: %v", err)
	}

	extractedFile := filepath.Join(dstDir, "greeting.txt")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist, but it does not", extractedFile)
	}
}

// TestTarExpander_Expand_TarBz2 tests extracting a simple .tar.bz2 file.
func TestTarExpander_Expand_TarBz2(t *testing.T) {
	tarExpander := &TarExpander{}

	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "test.tar.bz2")
	dstDir := filepath.Join(tempDir, "output")

	err := createTarBz2File(srcFile, "bz-file.txt", "Hello from tar.bz2!")
	if err != nil {
		t.Fatalf("failed to create tar.bz2 file: %v", err)
	}

	ctx := context.Background()
	err = tarExpander.Expand(ctx, srcFile, dstDir, 0)
	if err != nil {
		t.Fatalf("Expand returned an unexpected error: %v", err)
	}

	extractedFile := filepath.Join(dstDir, "bz-file.txt")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist, but it does not", extractedFile)
	}
}

// TestTarExpander_Expand_InvalidSource checks behavior when the source file doesn't exist.
func TestTarExpander_Expand_InvalidSource(t *testing.T) {
	tarExpander := &TarExpander{}

	tempDir := t.TempDir()
	nonExistentSrc := filepath.Join(tempDir, "does_not_exist.tar")
	dstDir := filepath.Join(tempDir, "output")

	ctx := context.Background()
	err := tarExpander.Expand(ctx, nonExistentSrc, dstDir, 0)
	if err == nil {
		t.Fatalf("expected error when source file does not exist")
	}
}

func TestTarExpander_Expand_PathTraversal(t *testing.T) {
	tarExpander := &TarExpander{}

	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "evil.tar")
	dstDir := filepath.Join(tempDir, "output")

	// safearchive sanitizes "../" prefixes, so the entry is extracted
	// with the traversal components stripped. IsPathContained provides
	// defense-in-depth if safearchive is ever bypassed.
	err := createTarFile(srcFile, "../../../etc/passwd", "root:x:0:0:root:/root:/bin/bash")
	if err != nil {
		t.Fatalf("failed to create tar file: %v", err)
	}

	ctx := context.Background()
	err = tarExpander.Expand(ctx, srcFile, dstDir, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	escaped := filepath.Join(tempDir, "etc", "passwd")
	if _, statErr := os.Stat(escaped); statErr == nil {
		t.Fatal("file should not have been written outside destination directory")
	}

	sanitized := filepath.Join(dstDir, "etc", "passwd")
	if _, statErr := os.Stat(sanitized); os.IsNotExist(statErr) {
		t.Fatal("safearchive should have sanitized the path and extracted the file inside dst")
	}
}

// tarEntry is a name/content pair for building multi-file test tars.
type tarEntry struct {
	Name    string
	Content string
}

// createTarFiles creates a .tar containing the given entries.
func createTarFiles(filePath string, entries []tarEntry) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: e.Name,
			Mode: 0600,
			Size: int64(len(e.Content)),
		}); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(e.Content)); err != nil {
			return err
		}
	}
	return nil
}

// createTarFile creates a simple .tar with one file.
func createTarFile(filePath string, fileName string, content string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	header := &tar.Header{
		Name: fileName,
		Mode: 0600,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err = tw.Write([]byte(content))
	return err
}

// createTarGzFile creates a simple .tar.gz with one file.
func createTarGzFile(filePath, fileName, content string) error {
	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: fileName,
		Mode: 0600,
		Size: int64(len(content)),
	}); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	_, err = io.Copy(gw, &buf)
	return err
}

// createTarBz2File creates a simple .tar.bz2 with one file.
func createTarBz2File(filePath, fileName, content string) error {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{
		Name: fileName,
		Mode: 0600,
		Size: int64(len(content)),
	}); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %w", filePath, err)
	}
	defer outFile.Close()

	bw, err := bzip2.NewWriter(outFile, nil)
	if err != nil {
		return fmt.Errorf("failed to create bzip2 writer: %w", err)
	}
	defer bw.Close()

	if _, err := io.Copy(bw, &tarBuf); err != nil {
		return fmt.Errorf("failed to write bzip2-compressed tar data: %w", err)
	}

	return nil
}
