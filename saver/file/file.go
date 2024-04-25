package file

import (
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

// FileSaver handles saving data to local filesystem paths.
type FileSaver struct{}

// Save implements the Saver interface for file destinations.
func (fs *FileSaver) Save(ctx context.Context, data io.Reader, destination string) error {

	dst, err := url.Parse(destination)
	if err != nil {
		return err
	}

	// Ensure the destination directory exists.
	if err := os.MkdirAll(filepath.Dir(dst.Path), 0755); err != nil {
		return err
	}

	// Create the destination file.
	f, err := os.Create(dst.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write the data to the file.
	_, err = io.Copy(f, data)
	return err
}
