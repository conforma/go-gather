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

// Package http provides methods for gathering HTTP resources.
// This package implements the Gatherer interface and provides methods for downloading files from HTTP sources,
// retrieving metadata of the downloaded file, and handling HTTP requests.
//
// The HTTPGatherer struct represents an HTTP gatherer and contains a http.Client for making HTTP requests.
// It implements the Gatherer interface's Gather method to download files from HTTP sources and return the metadata of the downloaded file.
//
// Example usage:
//
//	httpGatherer := http.NewHTTPGatherer()
//	metadata, err := httpGatherer.Gather(context.Background(), "http://example.com/file.txt", "/path/to/destination/with/optional/filename.txt")
//	if err != nil {
//	  fmt.Println("Error:", err)
//	  return
//	}
//	fmt.Println("Downloaded file metadata:", metadata)
//
// Note: The Gather method uses the http.Client's default timeout of 15 seconds for the HTTP requests.
// You can customize the timeout by modifying the http.Client's Timeout field before calling the Gather method.
package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	gogather "github.com/enterprise-contract/go-gather"
	"github.com/enterprise-contract/go-gather/metadata"
	httpMetadata "github.com/enterprise-contract/go-gather/metadata/http"
	"github.com/enterprise-contract/go-gather/saver"
)

var Transport http.RoundTripper = http.DefaultTransport

type HTTPGatherer struct {
	Client http.Client
}

func NewHTTPGatherer() *HTTPGatherer {
	return &HTTPGatherer{
		Client: http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (h *HTTPGatherer) Gather(ctx context.Context, source, destination string) (metadata.Metadata, error) {

	// Parse source
	src, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("error parsing source URI: %w", err)
	}

	// Check if the source scheme is provided
	if src.Scheme == "" {
		return nil, fmt.Errorf("no source scheme provided")
	}

	// Get the source filename
	sourceFileName := filepath.Base(src.Path)

	// Check if the source filename is provided
	if sourceFileName == "" {
		return nil, fmt.Errorf("specify a path to a file to download")
	}

	// Check if the destination has a trailing slash.
	// If it does, append the source filename to the destination path.
	if strings.HasSuffix(destination, "/") {
		destination = filepath.Join(destination, sourceFileName)
	} else {
		// If it doesn't, append the source filename to the destination path.
		if filepath.Ext(destination) == "" {
			destination = filepath.Join(destination, "/", sourceFileName)
		}
	}

	// Validate the destination path
	err = gogather.ValidateFileDestination(destination)
	if err != nil {
		return nil, fmt.Errorf("error validating destination: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Go-Gather")

	h.Client.Transport = Transport

	// Send the HTTP request
	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error downloading file: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response was successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response code error: %d", resp.StatusCode)
	}
	// Determine the destination type
	scheme, err := gogather.ClassifyURI(destination)
	if err != nil {
		return nil, fmt.Errorf("error determining destination type: %w", err)
	}

	// Create a new saver based on the destination scheme
	s, err := saver.NewSaver(scheme.String())
	if err != nil {
		return nil, fmt.Errorf("error creating saver: %w", err)
	}

	// Save the downloaded file
	err = s.Save(ctx, resp.Body, destination)
	if err != nil {
		if strings.Contains(err.Error(), "is a directory") {
			destination = filepath.Join(destination, filepath.Base(src.Path))
			err = s.Save(ctx, resp.Body, destination)
			if err != nil {
				return nil, fmt.Errorf("error saving file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("error saving file: %w", err)
		}
	}

	// Return the metadata of the downloaded file
	m := httpMetadata.HTTPMetadata{
		URL:           source,
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		Destination:   destination,
		Headers:       resp.Header,
	}
	return m, nil
}
