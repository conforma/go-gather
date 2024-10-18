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

package gogather

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// URLType is an enum for URL types
type URIType int

const (
	GitURI URIType = iota
	HTTPURI
	FileURI
	OCIURI
	K8SPURI
	Unknown
)

var getHomeDir = os.UserHomeDir

// String returns the string representation of the URLType
func (t URIType) String() string {
	return [...]string{"GitURI", "HTTPURI", "FileURI", "OCIURI", "K8SPURI", "Unknown"}[t]
}

// ExpandTilde expands a leading tilde in the file path to the user's home directory
func ExpandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := getHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// ClassifyURI classifies the input string as a Git URI, HTTP(S) URI, or file path
func ClassifyURI(input string) (URIType, error) {
	// Check for special prefixes first
	if strings.HasPrefix(input, "file::") {
		return FileURI, nil
	}
	if strings.HasPrefix(input, "git::") {
		return GitURI, nil
	}
	if strings.HasPrefix(input, "http::") {
		return HTTPURI, nil
	}

	if strings.HasPrefix(input, "oci::") {
		return OCIURI, nil
	}

	if strings.HasPrefix(input, "k8s*::") {
		return K8SPURI, nil
	}

	// Check for known git hosting services
	if strings.HasPrefix(input, "github.com") || strings.HasPrefix(input, "gitlab.com") {
		return GitURI, nil
	}

	// Check for schemes by trying to parse the input as a URL
	u, err := url.Parse(input)
	if err != nil {
		log.Printf("unable to parse input '%s' as URI: %v - this is just a notice, not an error", input, err)
	} else if u.Scheme != "" {
		switch u.Scheme {
		case "git":
			return GitURI, nil
		case "http", "https":
			return HTTPURI, nil
		case "file":
			return FileURI, nil
		case "oci":
			return OCIURI, nil
		// TODO: Not sure if this actually reachable when parsing as a URL.
		case "k8s*":
			return K8SPURI, nil
		}
	}

	// Regular expression for file paths
	filePathPattern := regexp.MustCompile(`^(\./|\../|/|[a-zA-Z]:\\|~\/|file://).*`)
	// Regular expression for Git URIs
	gitURIPattern := regexp.MustCompile(`^(git@.+|.+/[^/]*\.git(?:/.*|$))`)

	// Check if the input matches the file path pattern first
	if filePathPattern.MatchString(input) {
		// Expand the tilde in the file path if it exists
		input = ExpandTilde(input)
		// Check if the input ends with ".git" to classify as GitURI
		if strings.HasSuffix(input, ".git") {
			return GitURI, nil
		}
		return FileURI, nil
	}

	// Check if the input matches the Git URI pattern
	if gitURIPattern.MatchString(input) {
		return GitURI, nil
	}

	// Check if the input matches any known OCI registry
	if containsOCIRegistry(input) {
		return OCIURI, nil
	}

	// Check for unsupported schemes
	if err == nil && u.Scheme != "" {
		return Unknown, fmt.Errorf("unsupported protocol: %s", u.Scheme)
	}

	// Check if the input contains a dot but lacks a valid scheme
	if strings.Contains(input, ".") {
		return Unknown, fmt.Errorf("got %s. HTTP(S) URIs require a scheme (http:// or https://)", input)
	}

	return Unknown, nil
}

// ValidateFileDestination validates the destination path for saving files
func ValidateFileDestination(destination string) error {
	// Expand the tilde in the file path if it exists
	destination = ExpandTilde(destination)
	// Check if the destination file exists.
	_, err := os.Stat(destination)
	if err == nil {
		return fmt.Errorf("destination file already exists: %s", destination)
	}
	if os.IsNotExist(err) {
		return nil
	}
	return nil
}

// containsOCIRegistry checks if the input string contains a known OCI registry
func containsOCIRegistry(src string) bool {
	matchRegistries := []*regexp.Regexp{
		regexp.MustCompile("azurecr.io"),
		regexp.MustCompile("gcr.io"),
		regexp.MustCompile("registry.gitlab.com"),
		regexp.MustCompile("pkg.dev"),
		regexp.MustCompile("[0-9]{12}.dkr.ecr.[a-z0-9-]*.amazonaws.com"),
		regexp.MustCompile("^quay.io"),
		regexp.MustCompile(`(?:::1|127\.0\.0\.1|(?i:localhost)):\d{1,5}`), // localhost OCI registry
	}

	for _, matchRegistry := range matchRegistries {
		if matchRegistry.MatchString(src) {
			return true
		}
	}
	return false
}
