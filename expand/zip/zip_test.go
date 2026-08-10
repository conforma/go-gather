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

package zip_test

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	customzip "github.com/conforma/go-gather/expand/zip"
)

// TestZipExpander_Matcher verifies that the Matcher function correctly identifies .zip files.
func TestZipExpander_Matcher(t *testing.T) {
	z := &customzip.ZipExpander{}

	testCases := []struct {
		name string
		file string
		want bool
	}{
		{"zip extension", "archive.zip", true},
		{"no zip extension", "archive.tar", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := z.Matcher(tc.file)
			if got != tc.want {
				t.Errorf("Matcher(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

// TestZipExpander_Expand_SimpleZip checks extracting a simple .zip file with one small file.
func TestZipExpander_Expand_SimpleZip(t *testing.T) {
	z := &customzip.ZipExpander{}

	tempDir := t.TempDir()
	srcZip := filepath.Join(tempDir, "test.zip")
	dstDir := filepath.Join(tempDir, "output")

	if err := createZipFile(srcZip, []zipTestFile{
		{Name: "hello.txt", Content: "Hello, ZIP!"},
	}); err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	ctx := context.Background()
	if err := z.Expand(ctx, srcZip, dstDir, 0755); err != nil {
		t.Fatalf("Expand returned an unexpected error: %v", err)
	}

	extractedFile := filepath.Join(dstDir, "hello.txt")
	if _, err := os.Stat(extractedFile); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist after extraction, but it does not", extractedFile)
	}
}

// TestZipExpander_Expand_WithDirectories checks that directories are created properly.
func TestZipExpander_Expand_WithDirectories(t *testing.T) {
	z := &customzip.ZipExpander{}

	tempDir := t.TempDir()
	srcZip := filepath.Join(tempDir, "test_with_dirs.zip")
	dstDir := filepath.Join(tempDir, "output")

	files := []zipTestFile{
		{Name: "folder1/", IsDir: true},
		{Name: "folder1/nested.txt", Content: "Nested content"},
		{Name: "folder2/", IsDir: true},
		{Name: "folder2/another.txt", Content: "Another file"},
	}

	if err := createZipFile(srcZip, files); err != nil {
		t.Fatalf("failed to create zip file with directories: %v", err)
	}

	ctx := context.Background()
	if err := z.Expand(ctx, srcZip, dstDir, 0755); err != nil {
		t.Fatalf("Expand returned an unexpected error: %v", err)
	}

	checkPaths := []string{
		filepath.Join(dstDir, "folder1"),
		filepath.Join(dstDir, "folder1", "nested.txt"),
		filepath.Join(dstDir, "folder2"),
		filepath.Join(dstDir, "folder2", "another.txt"),
	}
	for _, p := range checkPaths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Fatalf("expected %s to exist after extraction, but it does not", p)
		}
	}
}

// TestZipExpander_Expand_SizeLimit checks that an error is raised if a file exceeds the size limit.
func TestZipExpander_Expand_SizeLimit(t *testing.T) {
	z := &customzip.ZipExpander{
		FileSizeLimit: 10, // artificially small limit
	}

	tempDir := t.TempDir()
	srcZip := filepath.Join(tempDir, "test_size_limit.zip")
	dstDir := filepath.Join(tempDir, "output")

	largeContent := "This file is definitely more than 10 bytes."
	if err := createZipFile(srcZip, []zipTestFile{
		{Name: "large.txt", Content: largeContent},
	}); err != nil {
		t.Fatalf("failed to create zip file with large file: %v", err)
	}

	ctx := context.Background()
	err := z.Expand(ctx, srcZip, dstDir, 0755)
	if err == nil {
		t.Fatalf("expected an error due to file size limit exceeded, but got nil")
	}
}

// TestZipExpander_Expand_InvalidSource checks that an error is returned if the source file does not exist.
func TestZipExpander_Expand_InvalidSource(t *testing.T) {
	z := &customzip.ZipExpander{}

	tempDir := t.TempDir()
	nonExistentZip := filepath.Join(tempDir, "does_not_exist.zip")
	dstDir := filepath.Join(tempDir, "output")

	ctx := context.Background()
	err := z.Expand(ctx, nonExistentZip, dstDir, 0755)
	if err == nil {
		t.Fatalf("expected error when source file doesn't exist")
	}
}

func TestZipExpander_Expand_PathTraversal(t *testing.T) {
	z := &customzip.ZipExpander{}

	tempDir := t.TempDir()
	srcZip := filepath.Join(tempDir, "evil.zip")
	dstDir := filepath.Join(tempDir, "output")

	// safearchive sanitizes "../" prefixes, so the entry is extracted
	// with the traversal components stripped. IsPathContained provides
	// defense-in-depth if safearchive is ever bypassed.
	if err := createZipFile(srcZip, []zipTestFile{
		{Name: "../../../etc/passwd", Content: "root:x:0:0:root:/root:/bin/bash"},
	}); err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	ctx := context.Background()
	err := z.Expand(ctx, srcZip, dstDir, 0755)
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

// zipTestFile is a simple struct for creating in-test ZIP files.
type zipTestFile struct {
	Name    string
	Content string
	IsDir   bool
}

// createZipFile creates a .zip file at zipPath containing the specified files.
func createZipFile(zipPath string, files []zipTestFile) error {
	outFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip output file: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	for _, f := range files {
		if f.IsDir {
			// Use a trailing slash in the file name to indicate a directory in a .zip
			if len(f.Name) > 0 && f.Name[len(f.Name)-1] != '/' {
				f.Name += "/"
			}

			_, err := zipWriter.CreateHeader(&zip.FileHeader{
				Name:   f.Name,
				Method: zip.Deflate,
			})
			if err != nil {
				return fmt.Errorf("failed to create directory entry %q: %w", f.Name, err)
			}
			continue
		}

		// Otherwise, create a regular file in the ZIP
		writer, err := zipWriter.Create(f.Name)
		if err != nil {
			return fmt.Errorf("failed to create file entry %q: %w", f.Name, err)
		}

		if _, err := io.WriteString(writer, f.Content); err != nil {
			return fmt.Errorf("failed to write file content for %q: %w", f.Name, err)
		}
	}

	return nil
}
