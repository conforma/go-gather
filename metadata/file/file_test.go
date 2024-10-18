// Copyright The Enterprise Contract Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0
package file

import (
	"testing"
	"time"

	"github.com/enterprise-contract/go-gather/metadata"
)

func TestFileMetadata_Get(t *testing.T) {
	testTime := time.Now()
	// Create a FileMetadata instance
	m := &FileMetadata{
		Size:      int64(100),
		Path:      "/path/to/file",
		Timestamp: testTime,
		SHA:       "ef4e93945f5b3d481abe655d6ce3870132994c0bd5840e312d7ac97cde021050",
	}

	// Call the Get method
	result := m.Get()

	// Assert the expected values
	expected := map[string]interface{}{
		"size":      int64(100),
		"path":      "/path/to/file",
		"timestamp": testTime,
		"sha":       "ef4e93945f5b3d481abe655d6ce3870132994c0bd5840e312d7ac97cde021050",
	}

	if len(result) != len(expected) {
		t.Errorf("unexpected result length: got %d, want %d", len(result), len(expected))
	}

	for key, value := range expected {
		if result[key] != value {
			t.Errorf("unexpected value for key '%s': got %v, want %v", key, result[key], value)
		}
	}
}

func TestDirectoryMetadata_Get(t *testing.T) {
	testTime := time.Now()
	// Create a FileMetadata instance
	m := &DirectoryMetadata{
		Size:      int64(100),
		Path:      "/path/to/dir/",
		Timestamp: testTime,
	}

	// Call the Get method
	result := m.Get()

	// Assert the expected values
	expected := map[string]interface{}{
		"size":      int64(100),
		"path":      "/path/to/dir/",
		"timestamp": testTime,
	}

	if len(result) != len(expected) {
		t.Errorf("unexpected result length: got %d, want %d", len(result), len(expected))
	}

	for key, value := range expected {
		if result[key] != value {
			t.Errorf("unexpected value for key '%s': got %v, want %v", key, result[key], value)
		}
	}
}

func TestFileMetadata_GetPinnedURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectedURL   string
		expectError   bool
		expectedError error
	}{
		{
			name:        "valid file URI",
			url:         "file:///path/to/policy",
			expectedURL: "file::/path/to/policy",
			expectError: false,
		},
		{
			name:        "empty URL",
			url:         "",
			expectedURL: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := FileMetadata{Path: tt.url}
			gotURL, err := m.GetPinnedURL()
			if (err != nil) != tt.expectError {
				t.Errorf("GetPinnedURL() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if gotURL != tt.expectedURL {
				t.Errorf("GetPinnedURL() gotURL = %v, expectedURL %v", gotURL, tt.expectedURL)
			}
		})
	}
}
func TestFileMetadata_GetPinnedUrl(t *testing.T) {
	testCases := []struct {
		name     string
		metadata metadata.Metadata
		expected string
		hasError bool
	}{
		{
			name:     "With no prefix",
			metadata: &FileMetadata{Path: "/path/to/policy.yaml"},
			expected: "file::/path/to/policy.yaml",
			hasError: false,
		},
		{
			name:     "With file::",
			metadata: &FileMetadata{Path: "file::/path/to/policy.yaml"},
			expected: "file::/path/to/policy.yaml",
			hasError: false,
		},
		{
			name:     "With file://",
			metadata: &FileMetadata{Path: "file:///path/to/policy.yaml"},
			expected: "file::/path/to/policy.yaml",
			hasError: false,
		},
		{
			name:     "With emmpty file path",
			metadata: &FileMetadata{},
			expected: "",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // Run tests in parallel where possible

			got, err := tc.metadata.GetPinnedURL()
			if (err != nil) != tc.hasError {
				t.Errorf("GetPinnedURL() \nerror = %v, \nexpected error = %v", err, tc.hasError)
				t.Fatalf("GetPinnedURL() \nerror = %v, \nexpected error = %v", err, tc.hasError)
			}
			if got != tc.expected {
				t.Errorf("GetPinnedURL() = %q\ninput = %#v\nexpected = %q\ngot = %q", got, tc.metadata, tc.expected, got)
			}
		})
	}
}
func TestDirectoryMetadata_GetPinnedURL(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectedURL   string
		expectError   bool
		expectedError string
	}{
		{
			name:        "properly structured file path",
			url:         "file:///path/to/policy",
			expectedURL: "file::/path/to/policy",
			expectError: false,
		},
		{
			name:          "empty file path",
			url:           "",
			expectedURL:   "",
			expectError:   true,
			expectedError: "empty file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := DirectoryMetadata{Path: tt.url}
			gotURL, err := m.GetPinnedURL()
			if tt.expectError && err != nil {
				if err.Error() != tt.expectedError {
					t.Errorf("GetPinnedURL() error = %v, expectedError %v", err, tt.expectedError)
				}
				return
			}
			if gotURL != tt.expectedURL {
				t.Errorf("GetPinnedURL() gotURL = %v, expectedURL %v", gotURL, tt.expectedURL)
			}
		})
	}
}
func TestDirectoryMetadata_GetPinnedUrl(t *testing.T) {
	testCases := []struct {
		name     string
		metadata metadata.Metadata
		expected string
		hasError bool
	}{
		{
			name:     "With no prefix",
			metadata: &DirectoryMetadata{Path: "/path/to/policy.yaml"},
			expected: "file::/path/to/policy.yaml",
			hasError: false,
		},
		{
			name:     "With file::",
			metadata: &DirectoryMetadata{Path: "file::/path/to/policy.yaml"},
			expected: "file::/path/to/policy.yaml",
			hasError: false,
		},
		{
			name:     "With file://",
			metadata: &DirectoryMetadata{Path: "file:///path/to/policy.yaml"},
			expected: "file::/path/to/policy.yaml",
			hasError: false,
		},
		{
			name:     "With emmpty file path",
			metadata: &DirectoryMetadata{},
			expected: "",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // Run tests in parallel where possible

			got, err := tc.metadata.GetPinnedURL()
			if (err != nil) != tc.hasError {
				t.Errorf("GetPinnedURL() \nerror = %v, \nexpected error = %v", err, tc.hasError)
				t.Fatalf("GetPinnedURL() \nerror = %v, \nexpected error = %v", err, tc.hasError)
			}
			if got != tc.expected {
				t.Errorf("GetPinnedURL() = %q\ninput = %#v\nexpected = %q\ngot = %q", got, tc.metadata, tc.expected, got)
			}
		})
	}
}
