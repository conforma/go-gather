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
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
)

func TestOCIGatherer_Matcher(t *testing.T) {
	g := &OCIGatherer{}

	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{"oci protocol slash slash", "oci://myregistry.example.com/repo", true},
		{"oci protocol double colon", "oci::myregistry.example.com/repo", true},
		{"no prefix", "myregistry.example.com/repo", false},
		{"other prefix", "http://myregistry.example.com/repo", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := g.Matcher(tc.uri)
			if got != tc.want {
				t.Errorf("Matcher(%q) = %v, want %v", tc.uri, got, tc.want)
			}
		})
	}
}

func TestOCIGatherer_Gather_Success(t *testing.T) {
	artifactRef := "127.0.0.1:5000/my-repo:latest"
	memoryStore := memory.New()

	err := pushTestArtifact(memoryStore, artifactRef, []byte("test data"))
	if err != nil {
		t.Fatalf("failed to push test artifact: %v", err)
	}

	oldOrasCopy := orasCopy
	defer func() { orasCopy = oldOrasCopy }()
	orasCopy = func(ctx context.Context, srcOras oras.ReadOnlyTarget, srcRef string, dstOras oras.Target, dstRef string, opts oras.CopyOptions) (v1.Descriptor, error) {
		if srcRef != artifactRef {
			return v1.Descriptor{}, fmt.Errorf("unexpected reference %s, want %s", srcRef, artifactRef)
		}

		return oras.Copy(ctx, memoryStore, artifactRef, dstOras, dstRef, opts)
	}

	g := &OCIGatherer{}

	dstDir := t.TempDir()

	ctx := context.Background()

	srcURI := "oci://" + artifactRef // e.g. "oci://localhost:5000/my-repo:latest"
	meta, err := g.Gather(ctx, srcURI, dstDir)
	if err != nil {
		t.Fatalf("Gather returned an error: %v", err)
	}

	ociMeta, ok := meta.(OCIMetadata)
	if !ok {
		t.Fatalf("expected *OCIMetadata, got %T", meta)
	}
	if ociMeta.Path != dstDir {
		t.Errorf("expected Path=%s, got %s", dstDir, ociMeta.Path)
	}
	if ociMeta.Digest == "" {
		t.Error("expected a Digest, got empty")
	}
	if ociMeta.Timestamp == "" {
		t.Error("expected a Timestamp, got empty")
	}
}

func TestOCIGatherer_Gather_CanceledContext(t *testing.T) {
	g := &OCIGatherer{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediate cancellation

	_, err := g.Gather(ctx, "oci://localhost:5000/repo", t.TempDir())
	if err == nil {
		t.Fatal("expected an error due to canceled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestOCIGatherer_Gather_InvalidRef(t *testing.T) {
	g := &OCIGatherer{}
	ctx := context.Background()

	// Provide a reference that fails parse
	srcURI := "oci://___invalid@@"

	_, err := g.Gather(ctx, srcURI, t.TempDir())
	if err == nil {
		t.Fatal("expected an error for invalid ref, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse reference") {
		t.Errorf("expected 'failed to parse reference' in error, got %v", err)
	}
}

func TestOCIGatherer_Gather_MissingArtifact(t *testing.T) {
	g := &OCIGatherer{}

	// Temporarily override orasCopy to force an error
	oldOrasCopy := orasCopy
	defer func() { orasCopy = oldOrasCopy }()

	orasCopy = func(ctx context.Context, src oras.ReadOnlyTarget, srcRef string, dst oras.Target, dstRef string, opts oras.CopyOptions) (v1.Descriptor, error) {
		return v1.Descriptor{}, fmt.Errorf("pulling policy: artifact not found")
	}

	ctx := context.Background()
	_, err := g.Gather(ctx, "oci://localhost:5000/no-such:latest", t.TempDir())
	if err == nil {
		t.Fatal("expected error about missing artifact, got nil")
	}
	if !strings.Contains(err.Error(), "pulling policy: artifact not found") {
		t.Errorf("expected artifact not found error, got %v", err)
	}
}

func TestOCIGatherer_Gather_CreateDirError(t *testing.T) {
	g := &OCIGatherer{}

	dstDir := "/root/somepath" // likely fails unless test runs as root

	ctx := context.Background()
	_, err := g.Gather(ctx, "oci://localhost:5000/repo:latest", dstDir)
	if err == nil {
		t.Fatal("expected directory creation error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("expected 'failed to create directory' in error, got %v", err)
	}
}

func TestOCIGatherer_ociURLParse(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"with double colon", "oci::myregistry.com/myrepo:tag", "myregistry.com/myrepo:tag"},
		{"with slash slash", "oci://myregistry.com/myrepo:tag", "myregistry.com/myrepo:tag"},
		{"no prefix", "myregistry.com/myrepo:tag", "myregistry.com/myrepo:tag"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ociURLParse(tc.source)
			if got != tc.want {
				t.Errorf("ociURLParse(%q) = %q, want %q", tc.source, got, tc.want)
			}
		})
	}
}

func TestOCIGatherer_Gather_ReplaceLocalhost(t *testing.T) {
	g := &OCIGatherer{}

	oldOrasCopy := orasCopy
	defer func() { orasCopy = oldOrasCopy }()

	var gotRef string
	orasCopy = func(ctx context.Context, src oras.ReadOnlyTarget, srcRef string, dst oras.Target, dstRef string, opts oras.CopyOptions) (v1.Descriptor, error) {
		gotRef = srcRef
		return v1.Descriptor{}, nil
	}

	ctx := context.Background()
	_, _ = g.Gather(ctx, "oci://localhost:5000/myrepo", t.TempDir())

	// Expect the final reference to have "127.0.0.1"
	if !strings.Contains(gotRef, "127.0.0.1") {
		t.Errorf("expected reference to contain 127.0.0.1, got %s", gotRef)
	}
}

func TestOCIGatherer_Gather_SetClientTransport(t *testing.T) {
	expectedMaxIdleConns := 200
	g := &OCIGatherer{}
	customTransport := &http.Transport{
		DisableKeepAlives: true,
		MaxIdleConns:    expectedMaxIdleConns,
	}

	// Set a custom transport
	g.SetClientTransport(customTransport)

	actualMaxIdleConns := g.Transport.(*http.Transport).MaxIdleConns
	//ensure that we have the same transport
	assert.Equal(t, actualMaxIdleConns, expectedMaxIdleConns, fmt.Sprintf("expected maxIdleConss to be %v, got %v", expectedMaxIdleConns, actualMaxIdleConns))
}

// pushTestArtifact stores data in a memory.Store under a final reference (e.g., "localhost:5000/my-repo:latest").
func pushTestArtifact(m *memory.Store, finalRef string, data []byte) error {
	ctx := context.Background()

	// 1. Build an OCI descriptor for this blob.
	d := digest.FromBytes(data)
	desc := v1.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    d,
		Size:      int64(len(data)),
	}

	// 2. Push the blob, storing by digest. This does NOT create a named reference.
	if err := m.Push(ctx, desc, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to push blob into memory store: %w", err)
	}

	// 3. Tag the blob inside memory store with an internal name so we can reference it.
	//    For example, "sourceRef" is any string you like.
	sourceRef := "my-blob-name"
	if err := m.Tag(ctx, desc, sourceRef); err != nil {
		return fmt.Errorf("failed to tag blob in memory store: %w", err)
	}

	// 4. Now we can oras.Copy from "my-blob-name" to the finalRef,
	//    effectively "tagging" the blob in the store as finalRef.
	_, err := oras.Copy(ctx, m, sourceRef, m, finalRef, oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("failed to alias data in memory store: %w", err)
	}

	return nil
}

// Optional TestOCIMetadata_Get to show retrieving the raw metadata structure
func TestOCIMetadata_Get(t *testing.T) {
	o := OCIMetadata{
		Path:      "/tmp/some/path",
		Digest:    "sha256:123abc",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	got := o.Get()
	if !reflect.DeepEqual(got, o) {
		t.Errorf("Get() = %+v, want same struct pointer %+v", got, o)
	}
}

// Optional TestOCIMetadata_GetDigest ensures the Digest is returned properly
func TestOCIMetadata_GetDigest(t *testing.T) {
	o := &OCIMetadata{
		Digest: "sha256:123abc",
	}
	got := o.GetDigest()
	if got != "sha256:123abc" {
		t.Errorf("GetDigest() = %q, want %q", got, "sha256:123abc")
	}
}
