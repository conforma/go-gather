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
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyDirConfined recursively copies the directory located at relPath (a path
// relative to root) into dst. Every read is resolved through an os.Root anchored
// at root, so any symlink or path component that would escape root is refused at
// the syscall level. Symlinks whose targets stay within root are followed
// normally, preserving legitimate in-tree links. dst is written with ordinary
// filesystem operations and may live outside root.
func CopyDirConfined(root, relPath, dst string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("failed to open confinement root %q: %w", root, err)
	}
	defer r.Close()

	// The top-level source must be a real directory, matching the contract of
	// the CopyDir this replaces. A symlinked subdir (to a file or directory) or
	// a plain file is rejected rather than copied.
	info, err := r.Lstat(relPath)
	if err != nil {
		return fmt.Errorf("failed to lstat %q within root: %w", relPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to copy symlinked subdirectory %q", relPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("source %q is not a directory", relPath)
	}

	return copyDirConfined(r, relPath, dst, info.Mode().Perm())
}

// copyDirConfined copies the directory at rel (relative to the confinement root
// r) into dst. Reads are resolved through r so that any symlink or path
// component escaping the root is refused at the syscall level.
//
// Symlinks encountered while recursing are handled explicitly: an in-tree
// symlink to a file is followed and its contents copied, but a symlink to a
// directory is refused rather than recursed into. Following directory symlinks
// would allow cycles (loop -> .) and exponential copy amplification via diamonds
// on this untrusted path, so they are not supported.
func copyDirConfined(r *os.Root, rel, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(dst, mode); err != nil {
		return fmt.Errorf("failed to create destination directory %q: %w", dst, err)
	}
	// MkdirAll applies the umask and leaves an existing directory's mode
	// untouched, so restore the source directory's mode explicitly.
	if err := os.Chmod(dst, mode); err != nil {
		return fmt.Errorf("failed to chmod destination directory %q: %w", dst, err)
	}

	dir, err := r.Open(rel)
	if err != nil {
		return fmt.Errorf("failed to open directory %q within root: %w", rel, err)
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return fmt.Errorf("failed to read directory %q: %w", rel, err)
	}

	for _, entry := range entries {
		childRel := filepath.Join(rel, entry.Name())
		childDst := filepath.Join(dst, entry.Name())

		// Lstat inspects the entry itself without following it, so symlinks can
		// be handled explicitly.
		info, err := r.Lstat(childRel)
		if err != nil {
			return fmt.Errorf("failed to lstat %q within root: %w", childRel, err)
		}

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			// Resolve the link within the root; an escaping target is rejected
			// here rather than being followed.
			target, err := r.Stat(childRel)
			if err != nil {
				return fmt.Errorf("failed to resolve symlink %q within root: %w", childRel, err)
			}
			if target.IsDir() {
				return fmt.Errorf("refusing to copy directory symlink %q", childRel)
			}
			if err := copyFileConfined(r, childRel, childDst, target.Mode()); err != nil {
				return err
			}
		case info.IsDir():
			if err := copyDirConfined(r, childRel, childDst, info.Mode().Perm()); err != nil {
				return err
			}
		default:
			if err := copyFileConfined(r, childRel, childDst, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFileConfined copies a single file at rel (relative to root r) into dst,
// replicating the source mode. The source is opened through r so that a symlink
// escaping the root is refused.
func copyFileConfined(r *os.Root, rel, dst string, mode os.FileMode) error {
	src, err := r.Open(rel)
	if err != nil {
		return fmt.Errorf("failed to open file %q within root: %w", rel, err)
	}
	defer src.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("failed to create destination file %q: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, src); err != nil {
		return fmt.Errorf("failed to copy contents to %q: %w", dst, err)
	}

	if err := os.Chmod(dst, mode.Perm()); err != nil {
		return fmt.Errorf("failed to chmod destination file %q: %w", dst, err)
	}
	return nil
}
