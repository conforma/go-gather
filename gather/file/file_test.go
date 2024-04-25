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
	if err.Error() != "parse \":\": missing protocol scheme" {
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
	if err.Error() != "parse \":\": missing protocol scheme" {
		t.Logf("Expected: %s, Got: %s", "parse :: missing protocol scheme", err.Error())
		t.Fail()
	}
}
