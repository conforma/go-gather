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

package helpers

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestCopyFile_Success checks that CopyFile copies a file correctly.
func TestCopyFile_Success(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "src.txt")
	dstFile := filepath.Join(tempDir, "dst.txt")

	content := []byte("Hello, CopyFile!")
	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	if err := CopyFile(srcFile, dstFile); err != nil {
		t.Fatalf("CopyFile returned error: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("expected content %q, got %q", content, got)
	}

	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("failed to stat source file: %v", err)
	}
	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("failed to stat destination file: %v", err)
	}
	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("expected file mode %v, got %v", srcInfo.Mode(), dstInfo.Mode())
	}
}

// TestCopyFile_MissingSource checks an error is returned when the source file does not exist.
func TestCopyFile_MissingSource(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "does_not_exist.txt")
	dstFile := filepath.Join(tempDir, "dst.txt")

	err := CopyFile(srcFile, dstFile)
	if err == nil {
		t.Fatal("expected an error when source file doesn't exist, got nil")
	}
	if !strings.Contains(err.Error(), "could not open source file") {
		t.Errorf("expected error mentioning 'could not open source file', got %v", err)
	}
}

// TestCopyDir_Success checks that CopyDir copies a directory with nested files and subdirectories.
func TestCopyDir_Success(t *testing.T) {
	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	dstDir := filepath.Join(tempDir, "dst")

	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("failed to create source directory: %v", err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	file1 := filepath.Join(srcDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0600); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0600); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir returned error: %v", err)
	}

	copiedFile1 := filepath.Join(dstDir, "file1.txt")
	copiedFile2 := filepath.Join(dstDir, "subdir", "file2.txt")

	if _, err := os.Stat(copiedFile1); os.IsNotExist(err) {
		t.Errorf("expected %s to exist, but it does not", copiedFile1)
	}
	if _, err := os.Stat(copiedFile2); os.IsNotExist(err) {
		t.Errorf("expected %s to exist, but it does not", copiedFile2)
	}
	data1, err := os.ReadFile(copiedFile1)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	data2, err := os.ReadFile(copiedFile2)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(data1) != "content1" {
		t.Errorf("expected 'content1', got %s", string(data1))
	}
	if string(data2) != "content2" {
		t.Errorf("expected 'content2', got %s", string(data2))
	}
}

// TestCopyDir_NotDirectory checks an error is returned if the source is not a directory.
func TestCopyDir_NotDirectory(t *testing.T) {
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "not_a_dir.txt")
	dstDir := filepath.Join(tempDir, "dst")

	if err := os.WriteFile(srcFile, []byte("data"), 0600); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	err := CopyDir(srcFile, dstDir)
	if err == nil {
		t.Fatal("expected error when source is not a directory, got nil")
	}
	if !strings.Contains(err.Error(), "source") || !strings.Contains(err.Error(), "is not a directory") {
		t.Errorf("expected 'is not a directory' error, got %v", err)
	}
}

// TestCopyReader_Success checks copying from an arbitrary reader into a file.
func TestCopyReader_Success(t *testing.T) {
	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "output.txt")

	data := "Hello from CopyReader!"
	reader := bytes.NewBufferString(data)

	if err := CopyReader(reader, dstFile, 0644, 0); err != nil {
		t.Fatalf("CopyReader returned error: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(got) != data {
		t.Errorf("expected %q, got %q", data, string(got))
	}

	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("failed to stat output file: %v", err)
	}
	if info.Mode() != 0644 {
		t.Errorf("expected file mode 0644, got %v", info.Mode())
	}
}

// TestCopyReader_SizeLimit ensures copying stops at the file size limit.
func TestCopyReader_SizeLimit(t *testing.T) {
	tempDir := t.TempDir()
	dstFile := filepath.Join(tempDir, "limited.txt")

	data := []byte("1234567890") // 10 bytes
	reader := bytes.NewBuffer(data)

	// Limit to 5 bytes
	if err := CopyReader(reader, dstFile, 0644, 5); err != nil {
		t.Fatalf("CopyReader returned error: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(got) != "12345" {
		t.Errorf("expected 5 bytes '12345', got %q", string(got))
	}
}

// TestExpandPath_NoTilde checks ExpandPath returns the path unchanged if no tilde is present.
func TestExpandPath_NoTilde(t *testing.T) {
	p := "/home/user/some/path"
	got, err := ExpandPath(p)
	if err != nil {
		t.Fatalf("ExpandPath returned error: %v", err)
	}
	if got != p {
		t.Errorf("expected %q, got %q", p, got)
	}
}

// TestExpandPath_Tilde checks that ExpandPath expands "~" properly.
func TestExpandPath_Tilde(t *testing.T) {
	origFunc := PathExpanderFunc
	defer func() { PathExpanderFunc = origFunc }()

	mockHome := "/mock/home"
	PathExpanderFunc = func(path string) (string, error) {
		if strings.HasPrefix(path, "~") {
			return filepath.Join(mockHome, path[1:]), nil
		}
		return path, nil
	}

	input := "~/myfolder"
	got, err := ExpandPath(input)
	if err != nil {
		t.Fatalf("ExpandPath returned error: %v", err)
	}
	want := filepath.Join(mockHome, "myfolder")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestExpandPath_Failure checks an error is bubbled up if userHomeDirFunc fails.
func TestExpandPath_Failure(t *testing.T) {
	origFunc := PathExpanderFunc
	defer func() { PathExpanderFunc = origFunc }()

	// Force an error
	PathExpanderFunc = func(path string) (string, error) {
		return "", fmt.Errorf("boom")
	}

	_, err := ExpandPath("~/some/path")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error containing 'boom', got %v", err)
	}
}

// TestGetDirectorySize_Success checks that GetDirectorySize calculates the total size.
func TestGetDirectorySize_Success(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "f1.txt")
	file2 := filepath.Join(tempDir, "f2.txt")
	data1 := []byte("abc")
	data2 := []byte("12345")
	if err := os.WriteFile(file1, data1, 0600); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, data2, 0600); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	subDir := filepath.Join(tempDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	file3 := filepath.Join(subDir, "f3.txt")
	data3 := []byte("zxy")
	if err := os.WriteFile(file3, data3, 0600); err != nil {
		t.Fatalf("failed to write file3: %v", err)
	}

	size, err := GetDirectorySize(tempDir)
	if err != nil {
		t.Fatalf("GetDirectorySize returned error: %v", err)
	}
	expected := int64(len(data1) + len(data2) + len(data3))
	if size != expected {
		t.Errorf("expected size %d, got %d", expected, size)
	}
}

// TestGetDirectorySize_BadPath checks an error is returned for a non-existent path.
func TestGetDirectorySize_BadPath(t *testing.T) {
	_, err := GetDirectorySize("/no/such/dir/123")
	if err == nil {
		t.Fatal("expected an error for non-existent path, got nil")
	}
	if !strings.Contains(err.Error(), "failed to expand directory path") &&
		!strings.Contains(err.Error(), "failed to walk directory") {
		t.Errorf("expected an error about 'failed to expand directory path' or 'failed to walk directory', got %v", err)
	}
}

// TestGetDirectorySize_ExpandPathError checks that if ExpandPath fails, GetDirectorySize fails too.
func TestGetDirectorySize_ExpandPathError(t *testing.T) {
	origFunc := PathExpanderFunc
	defer func() { PathExpanderFunc = origFunc }()

	PathExpanderFunc = func(path string) (string, error) {
		return "", fmt.Errorf("mock expand path error")
	}

	_, err := GetDirectorySize("~")
	if err == nil {
		t.Fatal("expected an error due to expand path error, got nil")
	}
	if !strings.Contains(err.Error(), "mock expand path error") {
		t.Errorf("expected 'mock expand path error', got %v", err)
	}
}

func TestIsPathContained(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		root string
		want bool
	}{
		{"child of root", "/tmp/repo/subdir", "/tmp/repo", true},
		{"nested child", "/tmp/repo/a/b/c", "/tmp/repo", true},
		{"dot-dot escapes", "/tmp/etc", "/tmp/repo", false},
		{"partial name match", "/tmp/repoextra/file", "/tmp/repo", false},
		{"root itself", "/tmp/repo", "/tmp/repo", false},
		{"uncleaned path", "/tmp/repo/./subdir", "/tmp/repo", true},
		{"dot-dot in path", "/tmp/repo/a/../b", "/tmp/repo", true},
		{"dot-dot escaping via clean", "/tmp/repo/../etc", "/tmp/repo", false},
		{"filesystem root child", "/child", "/", true},
		{"filesystem root itself", "/", "/", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsPathContained(tt.path, tt.root)
			if got != tt.want {
				t.Errorf("IsPathContained(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.want)
			}
		})
	}
}

// Test type checks: These are optional if you want to confirm function signatures haven't changed
func TestHelpersFunctionsAreExpectedTypes(t *testing.T) {
	var _ func(string, string) error = CopyDir
	var _ func(string, string) error = CopyFile
	var _ func(io.Reader, string, os.FileMode, int64) error = CopyReader
	var _ func(string) (string, error) = ExpandPath
	var _ func(string) (int64, error) = GetDirectorySize
	if reflect.ValueOf(CopyDir).Kind() != reflect.Func {
		t.Error("CopyDir must be a function")
	}
}
