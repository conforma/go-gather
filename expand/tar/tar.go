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

// Package tar implements an Expander for tar archives, including gzip and bzip2 compressed variants.
package tar

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/safearchive/tar"

	"github.com/conforma/go-gather/expand"
	"github.com/conforma/go-gather/internal/helpers"
)

var (
	pathExpanderFunc = helpers.ExpandPath
	extractTarGzFunc = extractTarGz
	extractTarBzFunc = extractTarBz
	untarFunc        = untar
)

// TarExpander extracts tar, tar.gz, and tar.bz2 archives.
type TarExpander struct {
	FileSizeLimit int64
	FilesLimit    int
}

// Expand extracts a tar archive from src into the dst directory.
func (t *TarExpander) Expand(ctx context.Context, src, dst string, umask os.FileMode) error {

	src, err := pathExpanderFunc(src)
	if err != nil {
		return fmt.Errorf("failed to expand source path: %w", err)
	}
	dst, err = pathExpanderFunc(dst)
	if err != nil {
		return fmt.Errorf("failed to expand destination path: %w", err)
	}

	input, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %s", src)
	}
	defer input.Close()

	if strings.Contains(src, "tar.gz") || strings.Contains(src, "tgz") {
		if err = extractTarGzFunc(input, dst, t.FileSizeLimit, t.FilesLimit); err != nil {
			return fmt.Errorf("failed to extract tar.gz file: %s", err)
		}
	} else if strings.Contains(src, "tar.bz2") || strings.Contains(src, "tbz2") {
		if err = extractTarBzFunc(input, dst, src, t.FileSizeLimit, t.FilesLimit); err != nil {
			return fmt.Errorf("failed to extract tar.bz2 file: %s", err)
		}
	} else {
		if err = untarFunc(input, dst, src, t.FileSizeLimit, t.FilesLimit); err != nil {
			return fmt.Errorf("failed to untar file: %s", err)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get destination directory size: %s", dst)
	}

	return nil
}

// Matcher returns true if the file name contains a tar-related extension.
func (t *TarExpander) Matcher(fileName string) bool {
	extensions := []string{"tar", "tgz", "tbz2"}
	for _, ext := range extensions {
		if strings.Contains(fileName, ext) {
			return true
		}
	}
	return false
}

// extractTarBz is a helper function that extracts a tarball compressed with bzip2 to a destination directory
func extractTarBz(input io.Reader, dst, src string, fileSizeLimit int64, filesLimit int) error {
	bzr := bzip2.NewReader(input)
	return untar(bzr, dst, src, fileSizeLimit, filesLimit)
}

// extractTarGz is a helper function that extracts a tarball compressed with gzip to a destination directory
func extractTarGz(input io.Reader, dst string, fileSizeLimit int64, filesLimit int) error {
	gzr, err := gzip.NewReader(input)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %s", err)
	}
	defer gzr.Close()

	return untar(gzr, dst, "", fileSizeLimit, filesLimit)
}

// writeTarFile creates path, copies from r into it, and returns any copy or close error.
func writeTarFile(fPath string, mode os.FileMode, r io.Reader) (err error) {
	outFile, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("error creating file (%s): %w", fPath, err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(outFile, r); err != nil {
		return fmt.Errorf("error extracting file (%s): %w", fPath, err)
	}
	return nil
}

// untar is a helper function that untars a tarball to a destination directory based on the provided options.
func untar(input io.Reader, dst, src string, fileSizeLimit int64, filesLimit int) error {
	tarReader := tar.NewReader(input)

	seenDirs := map[string]*tar.Header{}
	now := time.Now()

	var (
		totalFileSize int64
		filesCount    int
	)

	// Initialize a counter for headers processed
	headerCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			if headerCount == 0 {
				return fmt.Errorf("tar file is empty: %s", src)
			}
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar header: %w", err)
		}

		headerCount++

		// Validate the file count limit
		if filesLimit > 0 {
			filesCount++
			if filesCount > filesLimit {
				return fmt.Errorf("tar file contains more files than the %d allowed: %d", filesLimit, filesCount)
			}
		}

		// Skip extended headers
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}

		// Construct the file path safely to prevent Zip Slip
		fPath := filepath.Join(dst, header.Name) // #nosec G305 we're checking the path below
		if !helpers.IsPathContained(fPath, dst) {
			return fmt.Errorf("illegal file path: %s", fPath)
		}

		fileInfo := header.FileInfo()
		if !fileInfo.IsDir() {
			totalFileSize += fileInfo.Size()

			// Enforce file size limit
			if fileSizeLimit > 0 && totalFileSize > fileSizeLimit {
				return fmt.Errorf("tar file size exceeds the %d limit: %d", fileSizeLimit, totalFileSize)
			}
		}

		if fileInfo.IsDir() {
			// Create directories and store their headers for later permission/timestamp adjustment
			if err := os.MkdirAll(fPath, 0755); err != nil { // Use a reasonable default, e.g., 0755
				return fmt.Errorf("failed to create directory (%s): %w", fPath, err)
			}
			seenDirs[fPath] = header
			continue
		}

		// Ensure the parent directory exists
		destPath := filepath.Dir(fPath)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			if err := os.MkdirAll(destPath, 0755); err != nil { // Use a reasonable default
				return fmt.Errorf("failed to create directory (%s): %w", destPath, err)
			}
		}
		// Extract the file

		if err := writeTarFile(fPath, header.FileInfo().Mode(), tarReader); err != nil {
			return err
		}

		// Set file times
		aTime, mTime := now, now
		if !header.AccessTime.IsZero() {
			aTime = header.AccessTime
		}
		if !header.ModTime.IsZero() {
			mTime = header.ModTime
		}
		if err := os.Chtimes(fPath, aTime, mTime); err != nil {
			return fmt.Errorf("failed to change file times (%s): %w", fPath, err)
		}
	}

	// Adjust directory permissions and timestamps
	for path, dirHeader := range seenDirs {
		// Set permissions
		if err := os.Chmod(path, dirHeader.FileInfo().Mode()); err != nil {
			return fmt.Errorf("failed to change directory permissions (%s): %w", path, err)
		}

		// Set timestamps
		aTime, mTime := now, now
		if !dirHeader.AccessTime.IsZero() {
			aTime = dirHeader.AccessTime
		}
		if !dirHeader.ModTime.IsZero() {
			mTime = dirHeader.ModTime
		}
		if err := os.Chtimes(path, aTime, mTime); err != nil {
			return fmt.Errorf("failed to change directory times (%s): %w", path, err)
		}
	}

	return nil
}

func init() {
	expand.RegisterExpander(&TarExpander{})
}
