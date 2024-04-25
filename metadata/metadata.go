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

// Package metadata provides functionality for generating metadata.
// It includes a Metadata interface that contains a Describe method
// for describing metadata.

package metadata

// Metadata is an interface that all metadata types will satisfy.
type Metadata interface {
	Get() map[string]any // Example method; adjust according to actual use cases.
}
