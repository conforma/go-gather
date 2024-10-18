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
	"strings"
)

type OCIMetadata struct {
	URL    string
	Digest string
}

func (o OCIMetadata) Get() map[string]any {
	return map[string]any{
		"digest": o.Digest,
	}
}

// GetDigest returns the digest of the artifact.
func (o OCIMetadata) GetDigest() string {
	return o.Digest
}

func (o OCIMetadata) GetPinnedURL() (string, error) {
	u := o.URL
	if len(u) == 0 {
		return "", fmt.Errorf("empty URL")
	}
	if o.Digest == "" {
		return "", fmt.Errorf("image digest not set")
	}
	for _, scheme := range []string{"oci::", "oci://", "https://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	parts := strings.Split(u, "@")
	if len(parts) > 1 {
		u = parts[0]
	}
	return fmt.Sprintf("oci::%s@%s", u, o.Digest), nil
}
