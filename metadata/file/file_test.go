package file

import (
	"testing"
	"time"
)

func TestFileMetadata_Get(t *testing.T) {
	testTime := time.Now()
	// Create a FileMetadata instance
	m := &FileMetadata{
		Size:      int64(100),
		Path:      "/path/to/file",
		Timestamp: testTime,
		SHA:       "ef4e93945f5b3d481abe655d6ce3870132994c0bd5840e312d7ac97cde021050",
	}

	// Call the Describe method
	result := m.Get()

	// Assert the expected values
	expected := map[string]interface{}{
		"size":      int64(100),
		"path":      "/path/to/file",
		"timestamp": testTime,
		"sha":       "ef4e93945f5b3d481abe655d6ce3870132994c0bd5840e312d7ac97cde021050",
	}

	if len(result) != len(expected) {
		t.Errorf("unexpected result length: got %d, want %d", len(result), len(expected))
	}

	for key, value := range expected {
		if result[key] != value {
			t.Errorf("unexpected value for key '%s': got %v, want %v", key, result[key], value)
		}
	}
}

func TestDirectoryMetadata_Describe(t *testing.T) {
	testTime := time.Now()
	// Create a FileMetadata instance
	m := &DirectoryMetadata{
		Size:      int64(100),
		Path:      "/path/to/dir/",
		Timestamp: testTime,
	}

	// Call the Describe method
	result := m.Get()

	// Assert the expected values
	expected := map[string]interface{}{
		"size":      int64(100),
		"path":      "/path/to/dir/",
		"timestamp": testTime,
	}

	if len(result) != len(expected) {
		t.Errorf("unexpected result length: got %d, want %d", len(result), len(expected))
	}

	for key, value := range expected {
		if result[key] != value {
			t.Errorf("unexpected value for key '%s': got %v, want %v", key, result[key], value)
		}
	}
}
