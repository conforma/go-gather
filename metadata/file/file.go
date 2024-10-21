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

// Package file provides metadata structures and methods for files and directories.
//
// This package defines two types: FileMetadata and DirectoryMetadata,
// which represent the metadata of a file and a directory, respectively.
// Each type has fields for size, path, and timestamp.
//
// The FileMetadata and DirectoryMetadata types both have a Get method,
// which returns a map containing the metadata information.
// The Get method can be used to obtain a description
// of the metadata in a structured format.
//
// Example usage:
//
//	file := file.FileMetadata{
//	    Size:      1024,
//	    Path:      "/path/to/file.txt",
//	    Timestamp: time.Now(),
//	}
//	description := file.Get()
//	fmt.Println(description)
//
// Output:
//
//	map[size:1024 path:/path/to/file.txt timestamp:2022-01-01 12:00:00 +0000 UTC sha: 11d2c59b0f250e65fdc37b9524315338f2590e61c9d5ece4e0e7e32abe419fab]
package file

import (
	"fmt"
	"strings"
	"time"
)

type FileMetadata struct {
	Size      int64
	Path      string
	Timestamp time.Time
	SHA       string
}

type DirectoryMetadata struct {
	Size      int64
	Path      string
	Timestamp time.Time
}

func (m *FileMetadata) Get() map[string]any {
	return map[string]any{
		"size":      m.Size,
		"path":      m.Path,
		"timestamp": m.Timestamp,
		"sha":       m.SHA,
	}
}

func (m FileMetadata) GetPinnedURL() (string, error) {
	u := m.Path
	if len(u) == 0 {
		return "", fmt.Errorf("empty file path")
	}
	for _, scheme := range []string{"file::", "file://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	return "file::" + u, nil
}

func (m *DirectoryMetadata) Get() map[string]any {
	return map[string]any{
		"size":      m.Size,
		"path":      m.Path,
		"timestamp": m.Timestamp,
	}
}

func (m DirectoryMetadata) GetPinnedURL() (string, error) {
	u := m.Path
	if len(u) == 0 {
		return "", fmt.Errorf("empty file path")
	}
	for _, scheme := range []string{"file::", "file://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	return "file::" + u, nil
}
