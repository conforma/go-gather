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

// Package oci provides functionality for gathering files or directories from OCI (Open Container Initiative) sources.
// It includes an implementation of the Gatherer interface, OCIGatherer, which allows copying files or directories from an OCI source to a destination path.
// The Gather method in OCIGatherer takes a source path and a destination path, and returns the metadata of the gathered file or directory and any error encountered.
// This package also includes a helper function, ociURLParse, for parsing the source URI.
package oci

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"

	r "github.com/enterprise-contract/go-gather/gather/oci/internal/registry"
	"github.com/enterprise-contract/go-gather/metadata"
	"github.com/enterprise-contract/go-gather/metadata/oci"
)

var Transport http.RoundTripper = http.DefaultTransport

var orasCopy = oras.Copy

// OCIGatherer is a struct that implements the Gatherer interface
// and provides methods for gathering from OCI.
type OCIGatherer struct{}

// Gather copies a file or directory from the source path to the destination path.
// It returns the metadata of the gathered file or directory and any error encountered.
// Portions of this file are derivative from the open-policy-agent/conftest project.
func (f *OCIGatherer) Gather(ctx context.Context, source, destination string) (metadata.Metadata, error) {
	if strings.Contains(source, "localhost") {
		source = strings.ReplaceAll(source, "localhost", "127.0.0.1")
	}

	// Parse the source URI
	repo := ociURLParse(source)

	// Get the artifact reference
	ref, err := registry.ParseReference(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference: %w", err)
	}

	// If the reference is empty, set it to "latest"
	if ref.Reference == "" {
		ref.Reference = "latest"
		repo = ref.String()
	}

	// Create the repository client
	src, err := remote.NewRepository(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository client: %w", err)
	}

	// Setup the client for the repository
	if err := r.SetupClient(src, Transport); err != nil {
		return nil, fmt.Errorf("failed to setup repository client: %w", err)
	}

	// Create the destination directory
	if err := os.MkdirAll(destination, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file store
	fileStore, err := file.New(destination)
	if err != nil {
		return nil, fmt.Errorf("file store: %w", err)
	}
	defer fileStore.Close()

	// Copy the artifact to the file store
	a, err := orasCopy(ctx, src, repo, fileStore, "", oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("pulling policy: %w", err)
	}

	return &oci.OCIMetadata{URL: source, Digest: a.Digest.String()}, nil
}

func ociURLParse(source string) string {
	if strings.Contains(source, "::") {
		source = strings.Split(source, "::")[1]
	}

	scheme, src, found := strings.Cut(source, "://")
	if !found {
		src = scheme
	}
	return src
}
