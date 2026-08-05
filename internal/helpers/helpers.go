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

// Package helpers provides filesystem utilities for copying files, directories, and expanding paths.
package helpers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IsPathContained reports whether path is contained within root after
// cleaning both paths. It prevents path traversal attacks by ensuring
// the cleaned path starts with the cleaned root followed by a path
// separator. Note: this is a lexical check and does not resolve symlinks.
func IsPathContained(path, root string) bool {
	return strings.HasPrefix(filepath.Clean(path), filepath.Clean(root)+string(os.PathSeparator))
}

// CopyDir recursively copies the contents of the source directory (src)
// into the destination directory (dst). If dst does not exist, it will be created
// with the same permission bits as src. Subdirectories and files will be copied
// recursively. If src is not a directory, an error is returned.
//
// Note: This function does not preserve symlinks as symlinks—it follows them
// (via os.ReadDir’s behavior). Extended file attributes (xattrs) or other
// metadata beyond basic permissions are not preserved.
func CopyDir(src, dst string) error {
	// Clean the paths to normalize things like trailing slashes or ./ ..
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("error getting source directory info: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %q is not a directory", src)
	}

	// If the destination directory does not exist, create it using the source directory’s mode.
	if _, err := os.Stat(dst); err != nil {
		if os.IsNotExist(err) {
			if mkdirErr := os.MkdirAll(dst, srcInfo.Mode()); mkdirErr != nil {
				return fmt.Errorf("failed to create destination directory %q: %w", dst, mkdirErr)
			}
		} else {
			return fmt.Errorf("failed to determine if directory %q exists: %w", dst, err)
		}
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory contents of %q: %w", src, err)
	}

	// Recursively copy each entry in the source directory
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile copies a single file from src to dst. The destination file is
// created (or truncated if it exists) with the same permission bits as the source.
// If any I/O error occurs, the function returns an error.
// Note: Extended file attributes (xattrs), ACLs, or other metadata beyond basic
// UNIX permissions are not preserved by this approach.
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("could not open source file %q: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("could not create destination file %q: %w", dst, err)
	}
	defer dstFile.Close()

	// Perform the copy
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy contents from %q to %q: %w", src, dst, err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("could not stat source file %q: %w", src, err)
	}

	// Replicate the source file’s mode (permissions) on the destination
	if chmodErr := os.Chmod(dst, srcInfo.Mode()); chmodErr != nil {
		return fmt.Errorf("failed to chmod destination file %q: %w", dst, chmodErr)
	}
	return nil
}

// CopyReader copies from an arbitrary io.Reader (src) to a file at path dst.
// The newly created or truncated file is opened with the specified mode (OS file
// permissions). If fileSizeLimit > 0, only up to fileSizeLimit bytes are read
// from src. After the copy, the mode is applied to the file.
func CopyReader(src io.Reader, dst string, mode os.FileMode, fileSizeLimit int64) error {
	dstF, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", dst, err)
	}
	defer dstF.Close()

	// If a limit is set, wrap src in an io.LimitReader
	if fileSizeLimit > 0 {
		src = io.LimitReader(src, fileSizeLimit)
	}

	if _, err := io.Copy(dstF, src); err != nil {
		return fmt.Errorf("failed to copy to file %q: %w", dst, err)
	}

	if chmodErr := os.Chmod(dst, mode); chmodErr != nil {
		return fmt.Errorf("failed to set mode on file %q: %w", dst, chmodErr)
	}
	return nil
}

// ExpandPath expands a path starting with "~" to the current user’s home directory.
// If the path does not start with "~", it is returned unchanged. If the user’s
// home directory cannot be determined, an error is returned.
//
// Note: This only expands "~" for the current user. It does not handle
// "~otheruser" expansions.

// userHomeDirFunc references the actual os.UserHomeDir by default.
// We can override this in tests to simulate errors or special behaviors.
var userHomeDirFunc = os.UserHomeDir

// PathExpanderFunc expands a leading "~" to the current user's home directory. It does not handle "~otheruser" forms. It can be overridden in tests.
var PathExpanderFunc = func(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		homeDir, err := userHomeDirFunc()
		if err != nil {
			return "", fmt.Errorf("could not get user home directory: %w", err)
		}
		return filepath.Join(homeDir, path[1:]), nil
	}
	return path, nil
}

// ExpandPath expands a tilde-prefixed path using PathExpanderFunc.
func ExpandPath(path string) (string, error) {
	return PathExpanderFunc(path)
}

// GetDirectorySize returns the total size of all regular files (in bytes)
// contained in the specified directory (recursively). If the path starts with "~",
// it will be expanded via ExpandPath. If the path is invalid or an error occurs
// during traversal, an error is returned.
//
// Note: This function counts the sizes of files within the directory. It
// does not handle special file types (e.g. device files, symlinks to large directories)
// in a special manner—they’re counted or followed as normal by filepath.Walk.
func GetDirectorySize(dir string) (int64, error) {
	expandedDir, err := ExpandPath(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to expand directory path %q: %w", dir, err)
	}

	var size int64
	err = filepath.Walk(expandedDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// If there's an error while walking a particular file/dir, bubble that up.
			return walkErr
		}
		// If it's a regular file, add its size to the total
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to walk directory %q: %w", expandedDir, err)
	}
	return size, nil
}
