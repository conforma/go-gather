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

package saver

import (
	"context"
	"fmt"
	"io"

	"github.com/enteprise-contract/go-gather/saver/file"
)

// Saver is an interface for saving data to a destination.
type Saver interface {
	Save(ctx context.Context, data io.Reader, destination string) error
}

// NewSaver returns a Saver instance based on the destination protocol.
func NewSaver(protocol string) (Saver, error) {
	switch protocol {
	case "file":
		return &file.FileSaver{}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}
