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

// Package git provides metadata structure for cloned git repositories.
//
// The GitMetadata type has a Get method, which returns a map containing the metadata information.
// The Get method can be used to obtain a description of the metadata in a structured format.
//
// Example usage:
//
//	    git := git.GitMetadata{
//	        Size:      1024,
//	        Path:      "/path/to/file.txt",
//	        Timestamp: time.Now(),
//			   Commits: []object.Commit{...}
//	    }
//	    metadata := git.Get()
//	    fmt.Println(metadata)
//
// Output:
//
//	map[size:1024 path:/path/to/file.txt timestamp:2022-01-01 12:00:00 +0000 UTC commits:[{...}] path: size:0 timestamp:0001-01-01 00:00:00 +0000 UTC]
package git

import (
	"fmt"
	"strings"
)

// GitMetadata is a struct that represents the metadata of a git repository.
// It has fields for size, path, timestamp, and commits.
type GitMetadata struct {
	URL          string
	LatestCommit string
}

func (m GitMetadata) Get() map[string]any {
	return map[string]any{
		"latest_commit": m.LatestCommit,
	}
}

func (m GitMetadata) GetLatestCommit() string {
	return m.LatestCommit
}

func (m GitMetadata) GetPinnedURL() (string, error) {
	u := m.URL
	if len(u) == 0 {
		return "", fmt.Errorf("empty URL")
	}
	if m.LatestCommit == "" {
		return "", fmt.Errorf("latest commit not set")
	}
	for _, scheme := range []string{"git::", "git://", "https://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	if strings.HasPrefix(u, "git@") {
		u = strings.Replace(strings.Split(u, "git@")[1], ":", "/", 1)
	}
	return "git::" + strings.SplitN(u, "?ref=", 2)[0] + "?ref=" + m.LatestCommit, nil
}

func (m GitMetadata) RemoteRef() string {
	return ""
}
