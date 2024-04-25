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

// Package file provides functionality for copying files and directories.
// It includes a FileGatherer struct that implements the Gatherer interface
// and provides methods for gathering files and directories.
package file

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/enteprise-contract/go-gather/metadata/file"
	"github.com/enteprise-contract/go-gather/saver"
	"github.com/enterprise-contract/go-gather/metadata"
)

// FileGatherer is a struct that implements the Gatherer interface
// and provides methods for gathering files and directories.
type FileGatherer struct{}

// Gather copies a file or directory from the source path to the destination path.
// It returns the metadata of the gathered file or directory and any error encountered.
func (f *FileGatherer) Gather(ctx context.Context, source, destination string) (metadata.Metadata, error) {
	// Determine if we have a file or directory
	sourceKind, err := os.Stat(source)
	if err != nil {
		return nil, err
	}

	// If it's a directory, call copyDirectory, otherwise call copyFile
	if sourceKind.IsDir() {
		return f.copyDirectory(ctx, source, destination)
	} else {
		return f.copyFile(ctx, source, destination)
	}
}

func (f *FileGatherer) copyFile(ctx context.Context, source, destination string) (metadata.Metadata, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Open the source file.
	srcFile, err := os.Open(filepath.Clean(source))
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()

	// Use the appropriate Saver to handle storing the data.
	destFile, err := url.Parse(destination)
	if err != nil {
		return nil, err
	}

	saver, err := saver.NewSaver(destFile.Scheme)
	if err != nil {
		return nil, err
	}

	if err := saver.Save(ctx, srcFile, destination); err != nil {
		return nil, err
	}

	// Create metadata for the copied file.
	info, err := os.Stat(destFile.Path)
	if err != nil {
		return nil, err
	}

	fileSha, err := getFileSha(destFile.Path)
	if err != nil {
		return nil, err
	}

	return &file.FileMetadata{
		Size:      info.Size(),
		Path:      destination,
		Timestamp: info.ModTime(),
		SHA:       fileSha,
	}, nil
}

// copyDirectory copies a directory from the source path to the destination path.
// It walks through the directory tree, creates the corresponding directories in the destination path,
// and copies each file in the directory to the destination path.
// It limits the number of concurrent operations to 10 to avoid overwhelming system resources.
// It returns the metadata of the copied directory and any error encountered.

func (f *FileGatherer) copyDirectory(ctx context.Context, source, destination string) (metadata.Metadata, error) {
	src, err := url.Parse(source)
	if err != nil {
		return nil, err
	}
	dst, err := url.Parse(destination)
	if err != nil {
		return nil, err
	}

	errChan := make(chan error, 100) // Increased buffer size
	done := make(chan bool)
	semaphore := make(chan struct{}, 10) // Limit to 10 concurrent operations

	var wg sync.WaitGroup // Using a WaitGroup to manage concurrency

	go func() {
		defer close(done)
		filepath.Walk(src.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			relPath, err := filepath.Rel(src.Path, path)
			if err != nil {
				return err
			}

			destPath := filepath.Join(dst.Path, relPath)
			if info.IsDir() {
				if err := os.MkdirAll(destPath, 0755); err != nil {
					return err
				}
			} else {
				semaphore <- struct{}{}
				wg.Add(1)
				go func() {
					defer func() {
						<-semaphore
						wg.Done()
					}()
					srcFile, err := os.Open(filepath.Clean(path))
					if err != nil {
						errChan <- err
						return
					}
					defer srcFile.Close()

					saver, err := saver.NewSaver(dst.Scheme)
					if err != nil {
						errChan <- err
						return
					}

					if err := saver.Save(ctx, srcFile, destPath); err != nil {
						errChan <- err
						return
					}
				}()
			}
			return nil
		})
		wg.Wait()      // Wait for all goroutines to finish
		close(errChan) // Close the channel safely after all sends are done
	}()

	// Handle errors and completion
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}
	<-done
	return &file.DirectoryMetadata{
		Path:      dst.Path,
		Timestamp: time.Now(),
	}, nil
}

func getFileSha(path string) (string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
