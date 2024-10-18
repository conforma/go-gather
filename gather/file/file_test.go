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

package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFileGatherer_Gather(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary file inside the temporary directory
	tempFile, err := os.CreateTemp(tempDir, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer tempFile.Close()

	// Write some content to the temporary file
	content := []byte("test content")
	if _, err := tempFile.Write(content); err != nil {
		t.Fatal(err)
	}

	if err != nil {
		t.Fatal(err)
	}
	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when the source is a file
	sourceFile := tempFile.Name()
	destinationFile := filepath.Join(tempDir, "destination_file")
	_, err = gatherer.Gather(context.Background(), sourceFile, fmt.Sprintf("%s%s", "file://", filepath.Join(tempDir, "destination_file")))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Assert that the destination file exists
	if _, err := os.Stat(destinationFile); err != nil {
		t.Errorf("destination file does not exist: %v", err)
	}

	// Test when the source is a directory
	sourceDir := tempDir
	destinationDir := filepath.Join(tempDir, "destination_dir")
	_, err = gatherer.Gather(context.Background(), sourceDir, fmt.Sprintf("%s%s", "file://", destinationDir))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Assert that the destination directory exists
	if _, err := os.Stat(destinationDir); err != nil {
		t.Errorf("destination directory does not exist: %v", err)
	}
}

func TestFileGatherer_Gather_Error(t *testing.T) {
	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when os.Stat returns an error
	source := "nonexistent_file"
	destination := "destination_file"
	_, err := gatherer.Gather(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
}

// TestFileGatherer_URLParseError tests the error handling of the URL parsing
func TestFileGatherer_Gather_URLParseError(t *testing.T) {
	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when url.Parse returns an error
	source := ":"
	destination := "destination_file"
	_, err := gatherer.Gather(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if err.Error() != "failed to parse source URI: parse \":\": missing protocol scheme" {
		t.Logf("Expected: %s, Got: %s", "parse :: missing protocol scheme", err.Error())
		t.Fail()
	}
}

// TestFileGatherer_copyFile tests the copyFile method
func TestFileGatherer_copyFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary file inside the temporary directory
	tempFile, err := os.CreateTemp(tempDir, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer tempFile.Close()

	// Write some content to the temporary file
	content := []byte("test content")
	if _, err := tempFile.Write(content); err != nil {
		t.Fatal(err)
	}

	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when the source is a file
	sourceFile := tempFile.Name()
	destinationFile := filepath.Join(tempDir, "destination_file")
	_, err = gatherer.copyFile(context.Background(), sourceFile, fmt.Sprintf("%s%s", "file://", destinationFile))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Assert that the destination file exists
	if _, err := os.Stat(destinationFile); err != nil {
		t.Errorf("destination file does not exist: %v", err)
	}

	// Test when the source is a directory
	sourceDir := tempDir
	destinationDir := filepath.Join(tempDir, "destination_dir")
	_, err = gatherer.copyFile(context.Background(), sourceDir, fmt.Sprintf("%s%s", "file://", destinationDir))
	if err == nil {
		t.Error("expected an error, but got nil")
	}
}

// TestFileGatherer_copyFile_Source_URIParseError tests the error handling of the URL parsing
func TestFileGatherer_copyFile_Source_URIParseError(t *testing.T) {
	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when url.Parse returns an error
	source := ":"
	destination := "destination_file"
	_, err := gatherer.copyFile(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if err.Error() != "failed to parse source URI: parse \":\": missing protocol scheme" {
		t.Logf("Expected: %s, Got: %s", "parse :: missing protocol scheme", err.Error())
		t.Fail()
	}
}

// TestFileGatherer_copyFile_OpenSourceFileError tests the error handling of opening the source file
func TestFileGatherer_copyFile_OpenSourceFileError(t *testing.T) {
	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when os.Open returns an error
	source := "nonexistent_file"
	destination := "destination_file"
	_, err := gatherer.copyFile(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
}

// TestFileGatherer_copyFile_Destination_URIParseError tests the error handling of the URL parsing
func TestFileGatherer_copyFile_Destination_URIParseError(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary file inside the temporary directory
	tempFile, err := os.CreateTemp(tempDir, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer tempFile.Close()

	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when url.Parse returns an error
	source := tempFile.Name()
	destination := ":"
	_, err = gatherer.copyFile(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if err.Error() != "failed to parse destination URI: parse \":\": unknown protocol scheme" {
		t.Logf("Expected: %s, Got: %s", "failed to parse destination URI: parse \":\": missing protocol scheme", err.Error())
		t.Fail()
	}
}

// TestFileGatherer_copyFile_CreateSaverError tests the error handling of creating the saver
func TestFileGatherer_copyFile_CreateSaverError(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary file inside the temporary directory
	tempFile, err := os.CreateTemp(tempDir, "testfile")
	if err != nil {
		t.Fatal(err)
	}

	// Write some content to the temporary file
	content := []byte("test content")
	if _, err := tempFile.Write(content); err != nil {
		t.Fatal(err)
	}

	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when saver.NewSaver returns an error
	source := tempFile.Name()
	destination := "ftp://destination_file"
	_, err = gatherer.copyFile(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if got, expected := err.Error(), "failed to classify destination URI: unsupported protocol: ftp"; got != expected {
		t.Logf("Expected: %s, Got: %s", expected, got)
		t.Fail()
	}

}

func TestFileGatherer_copyDirectory_Source_URIParseError(t *testing.T) {
	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when url.Parse returns an error
	source := ":"
	destination := "destination_dir"
	_, err := gatherer.copyDirectory(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if err.Error() != "failed to parse source URI: parse \":\": missing protocol scheme" {
		t.Logf("Expected: %s, Got: %s", "parse :: missing protocol scheme", err.Error())
		t.Fail()
	}
}

func TestFileGatherer_copyDirectory_Destination_URIParseError(t *testing.T) {
	// Create a FileGatherer instance
	gatherer := &FileGatherer{}

	// Test when url.Parse returns an error
	source := "source_dir"
	destination := ":"
	_, err := gatherer.copyDirectory(context.Background(), source, destination)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if err.Error() != "failed to parse destination URI: parse \":\": missing protocol scheme" {
		t.Logf("Expected: %s, Got: %s", "failed to parse destination URI: parse \":\": missing protocol scheme", err.Error())
		t.Fail()
	}
}

// TestFileGatherer_getFileSha tests the getFileSha method
func TestFileGatherer_getFileSha(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary file inside the temporary directory
	tempFile, err := os.CreateTemp(tempDir, "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer tempFile.Close()

	// Write some content to the temporary file
	content := []byte("test content")
	if _, err := tempFile.Write(content); err != nil {
		t.Fatal(err)
	}

	// Test when the source is a file
	sourceFile := tempFile.Name()
	fileSha, err := getFileSha(sourceFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Assert that the fileSha is not empty
	if fileSha == "" {
		t.Error("expected fileSha to be non-empty, but got empty")
	}
}

// TestFileGatherer_getFileSha_OpenFileError tests the error handling of opening the file
func TestFileGatherer_getFileSha_OpenFileError(t *testing.T) {
	// Test when os.Open returns an error
	source := "nonexistent_file"
	_, err := getFileSha(source)
	if err == nil {
		t.Error("expected an error, but got nil")
	}
	if err.Error() != "failed to open file: open nonexistent_file: no such file or directory" {
		t.Logf("Expected: %s, Got: %s", "failed to open file: open nonexistent_file: no such file or directory", err.Error())
		t.Fail()
	}

}

func TestPinnedUrlRoundtrip(t *testing.T) {
	// setup
	tmp := t.TempDir()
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}

	file := &FileGatherer{}
	destination := filepath.Join(tmp, "destination")
	ctx := context.Background()
	m, err := file.Gather(ctx, source, destination)
	if err != nil {
		t.Fatal(err)
	}

	pinned, err := m.GetPinnedURL()
	if err != nil {
		t.Fatal(err)
	}

	_, err = file.Gather(ctx, pinned, destination)
	if err != nil {
		t.Fatal(err)
	}
}
