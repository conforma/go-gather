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

package k8s_p

type K8SPMetadata struct {
	URL string
	Ref string
}

func (o K8SPMetadata) Get() map[string]any {
	// TODO: Any useful info here?
	return map[string]any{}
}

func (o K8SPMetadata) GetPinnedURL() (string, error) {
	// TODO: meh?
	return o.URL, nil
}

func (o K8SPMetadata) RemoteRef() string {
	return o.Ref
}
