package saver

import (
	"fmt"
	"testing"

	"github.com/enteprise-contract/go-gather/saver/file"
)

func TestNewSaver(t *testing.T) {
	// Test case 1: protocol is "file"
	protocol := "file"
	saver, err := NewSaver(protocol)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_, ok := saver.(*file.FileSaver)
	if !ok {
		t.Errorf("unexpected saver type: got %T, want *file.FileSaver", saver)
	}

	// Test case 2: unsupported protocol
	protocol = "unsupported"
	_, err = NewSaver(protocol)
	expectedErr := fmt.Errorf("unsupported protocol: %s", protocol)
	if err == nil {
		t.Error("expected an error, but got nil")
	} else if err.Error() != expectedErr.Error() {
		t.Errorf("unexpected error: got %v, want %v", err, expectedErr)
	}
}
