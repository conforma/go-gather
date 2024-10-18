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

package http

import (
	"fmt"
	"strings"
)

type HTTPMetadata struct {
	URL           string
	StatusCode    int
	ContentLength int64
	Destination   string
	Headers       map[string][]string
}

func (m HTTPMetadata) Get() map[string]any {
	return map[string]any{
		"statusCode":    m.StatusCode,
		"contentLength": m.ContentLength,
		"destination":   m.Destination,
		"headers":       m.Headers,
	}
}

func (m HTTPMetadata) GetPinnedURL() (string, error) {
	u := m.URL
	if len(u) == 0 {
		return "", fmt.Errorf("empty URL")
	}
	for _, scheme := range []string{"http://", "https://", "http::"} {
		u = strings.TrimPrefix(u, scheme)
	}
	return "http::" + u, nil
}
