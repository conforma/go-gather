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

package oci

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/enterprise-contract/go-gather/metadata"
)

func TestOCIMetadata_Get(t *testing.T) {
	o := OCIMetadata{Digest: "fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f"}
	expected := map[string]any{
		"digest": "fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
	}
	result := o.Get()
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected Get() to return %v, but got %v", expected, result)
	}
}

func TestOCIMetadata_GetDigest(t *testing.T) {
	o := OCIMetadata{Digest: "fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f"}
	expected := "fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f"
	result := o.GetDigest()
	assert.Equal(t, expected, result, "Expected GetDigest() to return %s, but got %s", expected, result)
}

func TestGetPinnedUrl(t *testing.T) {
	goodMetadata := OCIMetadata{
		Digest: "SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
	}
	badMetadata := OCIMetadata{}

	tests := []struct {
		name          string
		url           string
		expectedURL   string
		expectError   bool
		expectedError string
		metadata      OCIMetadata
	}{
		{
			name:        "valid URL with oci:// scheme",
			url:         "oci://example.com/org/repo",
			expectedURL: "oci::example.com/org/repo@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectError: false,
			metadata:    goodMetadata,
		},
		{
			name:        "valid URL with oci:: scheme",
			url:         "oci://example.com/org/repo",
			expectedURL: "oci::example.com/org/repo@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectError: false,
			metadata:    goodMetadata,
		},

		{
			name:        "valid URL with oci:// scheme and tag",
			url:         "oci://example.com/org/repo:latest",
			expectedURL: "oci::example.com/org/repo:latest@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectError: false,
			metadata:    goodMetadata,
		},
		{
			name:        "valid URL with oci:: scheme and tag",
			url:         "oci://example.com/org/repo:latest",
			expectedURL: "oci::example.com/org/repo:latest@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectError: false,
			metadata:    goodMetadata,
		},
		{
			name:        "valid URL with oci:: scheme, tag, and digest",
			url:         "oci://example.com/org/repo:latest@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectedURL: "oci::example.com/org/repo:latest@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectError: false,
			metadata:    goodMetadata,
		},
		{
			name:          "invalid URL",
			url:           "",
			expectedURL:   "oci://example.com/org/repo:latest@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectError:   true,
			expectedError: "empty URL",
			metadata:      goodMetadata,
		},
		{
			name:          "valid URL with empty metadata",
			url:           "oci://example.com/org/repo:latest",
			expectedURL:   "oci://example.com/org/repo:latest@SHA256:fa93b01658e3a5a1686dc3ae55f170d8de487006fb53a28efcd12ab0710a2e5f",
			expectError:   true,
			expectedError: "image digest not set",
			metadata:      badMetadata,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.metadata.GetPinnedURL(tt.url)
			if tt.expectError && err != nil {
				assert.Equal(t, err.Error(), tt.expectedError, fmt.Sprintf("GetPinnedURL() error = %v, expectedError %v", err, tt.expectedError))
				return
			}
			assert.Equal(t, result, tt.expectedURL, fmt.Sprintf("GetPinnedURL() gotURL = %v, expectedURL %v", result, tt.expectedURL))
		})
	}
}

func TestGetPinnedURL(t *testing.T) {
	testCases := []struct {
		name     string
		url      string
		metadata metadata.Metadata
		expected string
		hasError bool
	}{
		// OCI Metadata Tests
		{
			name: "oci:: prefix and repo tag",
			url:  "oci::registry/policy:latest",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy:latest@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "oci:// prefix and repo tag",
			url:  "oci://registry/org/policy:dev",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/org/policy:dev@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "oci:: prefix, path suffix, and repo tag",
			url:  "oci::registry/policy:main",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy:main@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "oci:: prefix and path suffix without repo tag",
			url:  "oci::registry/policy",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "no prefix, with repo tag",
			url:  "registry/policy:latest",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy:latest@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "no prefix, without repo tag",
			url:  "registry/policy",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "oci:: prefix, with path suffix, without tag",
			url:  "oci://registry/policy",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "oci:// prefix, with repo tag, with existing digest",
			url:  "oci://registry/policy:bar@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy:bar@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "oci:: prefix, with path suffix, with existing digest",
			url:  "oci::registry/policy:baz@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy:baz@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
		{
			name: "oci:: prefix, with path suffix, without tag",
			url:  "oci::registry/policy",
			metadata: &OCIMetadata{
				Digest: "sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			},
			expected: "oci::registry/policy@sha256:c04c1f5ea75e869e2da7150c927d0c8649790b2e3c82e6ff317d4cfa068c1649",
			hasError: false,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel() // Run tests in parallel where possible

			got, err := tc.metadata.GetPinnedURL(tc.url)
			if (err != nil) != tc.hasError {
				t.Errorf("GetPinnedURL() \nerror = %v, \nexpected error = %v", err, tc.hasError)
				t.Fatalf("GetPinnedURL() \nerror = %v, \nexpected error = %v", err, tc.hasError)
			}
			if got != tc.expected {
				t.Errorf("GetPinnedURL() = %q\ninput = %q\nexpected = %q\ngot = %q", got, tc.url, tc.expected, got)
			}
		})
	}
}
