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
	"os"
	"path/filepath"
	"testing"
)

// TestCopyDirConfined_RefusesNestedSymlinkEscapingRoot verifies that a symlink
// nested inside the copied tree whose target is outside the confinement root is
// refused, and that the target's contents are not copied into the destination.
func TestCopyDirConfined_RefusesNestedSymlinkEscapingRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0600); err != nil {
		t.Fatalf("failed to create secret file: %v", err)
	}

	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(sub, "leak")); err != nil {
		t.Fatalf("failed to create escaping symlink: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")

	err := CopyDirConfined(root, "sub", dst)
	if err == nil {
		t.Fatalf("expected CopyDirConfined to refuse an escaping symlink, got nil")
	}

	if _, statErr := os.Stat(filepath.Join(dst, "leak")); statErr == nil {
		t.Errorf("escaping symlink target was copied into dst; want it refused")
	}
}

// TestCopyDirConfined_RefusesSubdirSymlinkEscapingRoot verifies that when the
// selected relPath is itself a symlink pointing outside the root, the copy is
// refused rather than dereferencing the link.
func TestCopyDirConfined_RefusesSubdirSymlinkEscapingRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("top secret"), 0600); err != nil {
		t.Fatalf("failed to create secret file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "policies")); err != nil {
		t.Fatalf("failed to create escaping symlink: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")

	if err := CopyDirConfined(root, "policies", dst); err == nil {
		t.Fatalf("expected CopyDirConfined to refuse an escaping subdir symlink, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(dst, "secret.txt")); statErr == nil {
		t.Errorf("escaping subdir symlink was dereferenced into dst; want it refused")
	}
}

// TestCopyDirConfined_CopiesPlainTree verifies that a normal nested directory
// tree is copied faithfully, contents and file mode included.
func TestCopyDirConfined_CopiesPlainTree(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "policies", "nested")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "policies", "top.txt"), []byte("top"), 0600); err != nil {
		t.Fatalf("failed to write top file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("deep"), 0600); err != nil {
		t.Fatalf("failed to write deep file: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyDirConfined(root, "policies", dst); err != nil {
		t.Fatalf("CopyDirConfined returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "nested", "deep.txt"))
	if err != nil {
		t.Fatalf("failed to read copied nested file: %v", err)
	}
	if string(got) != "deep" {
		t.Errorf("nested file content = %q, want %q", got, "deep")
	}

	info, err := os.Stat(filepath.Join(dst, "top.txt"))
	if err != nil {
		t.Fatalf("failed to stat copied top file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("top file mode = %o, want %o", info.Mode().Perm(), 0600)
	}
}

// TestCopyDirConfined_FollowsInTreeSymlinkToFile verifies that a symlink whose
// target stays within the root is followed and its contents copied.
func TestCopyDirConfined_FollowsInTreeSymlinkToFile(t *testing.T) {
	root := t.TempDir()
	policies := filepath.Join(root, "policies")
	if err := os.MkdirAll(policies, 0755); err != nil {
		t.Fatalf("failed to create policies dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "real.txt"), []byte("in-tree"), 0600); err != nil {
		t.Fatalf("failed to write real file: %v", err)
	}
	// policies/link.txt -> ../real.txt (stays within root)
	if err := os.Symlink(filepath.Join("..", "real.txt"), filepath.Join(policies, "link.txt")); err != nil {
		t.Fatalf("failed to create in-tree symlink: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyDirConfined(root, "policies", dst); err != nil {
		t.Fatalf("CopyDirConfined returned error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatalf("failed to read copied in-tree symlink: %v", err)
	}
	if string(got) != "in-tree" {
		t.Errorf("in-tree symlink content = %q, want %q", got, "in-tree")
	}
}

// TestCopyDirConfined_RestoresExistingDestinationDirMode verifies that the
// destination directory ends with the source directory's mode even when the
// destination already exists (os.MkdirAll leaves an existing directory's mode
// untouched, so the copy must restore it explicitly).
func TestCopyDirConfined_RestoresExistingDestinationDirMode(t *testing.T) {
	root := t.TempDir()
	policies := filepath.Join(root, "policies")
	if err := os.MkdirAll(policies, 0755); err != nil {
		t.Fatalf("failed to create policies dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(policies, "p.rego"), []byte("x"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	// Pre-create dst with a mode different from the source directory's.
	if err := os.MkdirAll(dst, 0700); err != nil {
		t.Fatalf("failed to pre-create dst: %v", err)
	}

	if err := CopyDirConfined(root, "policies", dst); err != nil {
		t.Fatalf("CopyDirConfined returned error: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("dst dir mode = %o, want %o (source mode should be restored)", info.Mode().Perm(), 0755)
	}
}

// TestCopyDirConfined_FailsClosedOnBadInput verifies that a missing root or a
// missing subdir yields an error rather than silently succeeding.
func TestCopyDirConfined_FailsClosedOnBadInput(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out")

	if err := CopyDirConfined(filepath.Join(t.TempDir(), "nope"), "policies", dst); err == nil {
		t.Errorf("expected error for a non-existent confinement root, got nil")
	}

	root := t.TempDir()
	if err := CopyDirConfined(root, "missing", dst); err == nil {
		t.Errorf("expected error for a non-existent subdir, got nil")
	}
}

// TestCopyDirConfined_RefusesParentTraversal guards the confinement contract: a
// relPath that escapes the root via ".." must be refused and must not create the
// destination, so a regression to lexical parent traversal is caught.
func TestCopyDirConfined_RefusesParentTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatalf("failed to create root: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "out")

	if err := CopyDirConfined(root, "..", dst); err == nil {
		t.Fatalf("expected CopyDirConfined to refuse a '..' relPath, got nil")
	}
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Errorf("destination should not be created for a refused '..' relPath")
	}
}

// TestCopyDirConfined_RefusesDirectorySymlink verifies that a directory symlink
// (even one that stays within the root) is refused rather than recursed into.
// Following directory symlinks would allow cycles and exponential copy
// amplification via diamonds, so on the untrusted git path they are rejected.
// In-tree file symlinks remain supported.
func TestCopyDirConfined_RefusesDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "shared"), 0755); err != nil {
		t.Fatalf("failed to create shared dir: %v", err)
	}
	policies := filepath.Join(root, "policies")
	if err := os.MkdirAll(policies, 0755); err != nil {
		t.Fatalf("failed to create policies dir: %v", err)
	}
	// policies/current -> ../shared (in-tree symlink to a directory)
	if err := os.Symlink(filepath.Join("..", "shared"), filepath.Join(policies, "current")); err != nil {
		t.Fatalf("failed to create in-tree dir symlink: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyDirConfined(root, "policies", dst); err == nil {
		t.Fatalf("expected CopyDirConfined to refuse a directory symlink, got nil")
	}
}

// TestCopyDirConfined_RefusesSymlinkLoop verifies that a directory symlink that
// forms a cycle within the root is refused (it is a directory symlink) and does
// not recurse without bound.
func TestCopyDirConfined_RefusesSymlinkLoop(t *testing.T) {
	root := t.TempDir()
	policies := filepath.Join(root, "policies")
	if err := os.MkdirAll(policies, 0755); err != nil {
		t.Fatalf("failed to create policies dir: %v", err)
	}
	// policies/loop -> . (points back at policies itself, a cycle)
	if err := os.Symlink(".", filepath.Join(policies, "loop")); err != nil {
		t.Fatalf("failed to create loop symlink: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyDirConfined(root, "policies", dst); err == nil {
		t.Fatalf("expected CopyDirConfined to refuse a symlink cycle, got nil")
	}
}

// TestCopyDirConfined_RefusesNonDirectoryTopLevel verifies that the top-level
// relPath must be a directory, matching the contract of the CopyDir it replaces:
// selecting a file (or a symlink) as the subdir is rejected rather than copied.
func TestCopyDirConfined_RefusesNonDirectoryTopLevel(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("data"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyDirConfined(root, "file.txt", dst); err == nil {
		t.Fatalf("expected CopyDirConfined to reject a non-directory subdir, got nil")
	}
	if _, statErr := os.Stat(dst); statErr == nil {
		t.Errorf("destination should not be created for a non-directory subdir")
	}
}
