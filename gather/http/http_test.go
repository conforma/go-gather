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
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestHTTPGatherer_Gather_FileMode verifies a new download honors the process
// umask (matching os.Create), rather than being forced to a fixed mode: a
// typical umask yields group/world-readable files, a restrictive one does not.
func TestHTTPGatherer_Gather_FileMode(t *testing.T) {
	cases := []struct {
		name  string
		umask int
		want  os.FileMode
	}{
		{"typical umask", 0o022, 0o644},
		{"restrictive umask", 0o077, 0o600},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := syscall.Umask(tc.umask)
			defer syscall.Umask(old)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("hello"))
			})
			server := httptest.NewServer(handler)
			defer server.Close()

			g := NewHTTPGatherer()
			dest := filepath.Join(t.TempDir(), "file.txt")
			if _, err := g.Gather(context.Background(), server.URL+"/file.txt", dest); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			fi, err := os.Stat(dest)
			if err != nil {
				t.Fatalf("failed to stat destination: %v", err)
			}
			if fi.Mode().Perm() != tc.want {
				t.Errorf("downloaded file mode = %o, want %o (umask %o)", fi.Mode().Perm(), tc.want, tc.umask)
			}
		})
	}
}

func TestHTTPGatherer_Gather_PreservesExistingMode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new content"))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "file.txt")
	// 0400 is distinct from both the 0644 default and os.CreateTemp's 0600, so
	// this genuinely proves the existing mode is preserved.
	if err := os.WriteFile(dest, []byte("old"), 0400); err != nil {
		t.Fatalf("failed to seed destination: %v", err)
	}

	g := NewHTTPGatherer()
	if _, err := g.Gather(context.Background(), server.URL+"/file.txt", dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}
	if fi.Mode().Perm() != 0400 {
		t.Errorf("overwritten file mode = %o, want preserved %o", fi.Mode().Perm(), 0400)
	}
}

func TestHTTPGatherer_Gather_MaxInt64LimitNotEmpty(t *testing.T) {
	const body = "hello world"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	// A maximal positive limit must behave as effectively unlimited, not
	// overflow the internal limit+1 and silently produce an empty file.
	g := NewHTTPGatherer(WithMaxResponseBytes(math.MaxInt64))
	dest := filepath.Join(t.TempDir(), "file.txt")
	if _, err := g.Gather(context.Background(), server.URL+"/file.txt", dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(got) != body {
		t.Errorf("content = %q, want %q (limit+1 overflow truncated the body)", got, body)
	}
}

// TestHTTPGatherer_Gather_ChunkedBodyLimit streams a body with no Content-Length
// (chunked), so the early Content-Length reject is bypassed and the LimitReader
// path — the real defense against a misbehaving server — enforces the limit.
func TestHTTPGatherer_Gather_ChunkedBodyLimit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 4; i++ { // 4 x 256 = 1024 bytes, no Content-Length
			_, _ = w.Write([]byte(strings.Repeat("A", 256)))
			if flusher != nil {
				flusher.Flush()
			}
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	g := NewHTTPGatherer(WithMaxResponseBytes(512))
	dest := filepath.Join(t.TempDir(), "file.txt")

	_, err := g.Gather(context.Background(), server.URL+"/file.txt", dest)
	if err == nil {
		t.Fatalf("expected error for oversized chunked body, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size-limit error, got %v", err)
	}
}

// TestHTTPGatherer_Gather_LimitDisabled verifies that WithMaxResponseBytes(0)
// disables the limit. A guard inversion would instead reject any non-empty body,
// so this covers the disabled branch meaningfully.
func TestHTTPGatherer_Gather_LimitDisabled(t *testing.T) {
	body := strings.Repeat("A", 4096)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	g := NewHTTPGatherer(WithMaxResponseBytes(0))
	dest := filepath.Join(t.TempDir(), "file.txt")

	if _, err := g.Gather(context.Background(), server.URL+"/file.txt", dest); err != nil {
		t.Fatalf("unexpected error with limit disabled: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(got) != body {
		t.Errorf("content length = %d, want %d (limit should be disabled)", len(got), len(body))
	}
}

func TestHTTPGatherer_WithTransport(t *testing.T) {
	customTransport := &http.Transport{
		DisableKeepAlives: true,
	}
	g := NewHTTPGatherer(WithTransport(customTransport))

	if g.Client.Transport != customTransport {
		t.Errorf("expected custom transport, got %v", g.Client.Transport)
	}
}

func TestHTTPGatherer_DefaultTransport(t *testing.T) {
	g := NewHTTPGatherer()

	if g.Client.Transport != nil {
		t.Errorf("expected nil (Go default) transport, got %v", g.Client.Transport)
	}
}

func TestHTTPGatherer_Matcher(t *testing.T) {
	t.Parallel()
	g := &HTTPGatherer{}

	testCases := []struct {
		name string
		uri  string
		want bool
	}{
		{"http scheme", "http://example.com/file.txt", true},
		{"https scheme", "https://example.com/file.txt", true},
		{"no scheme", "example.com/file.txt", false},
		{"ftp scheme", "ftp://example.com/file.txt", false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := g.Matcher(tc.uri)
			if got != tc.want {
				t.Errorf("Matcher(%q) = %v, want %v", tc.uri, got, tc.want)
			}
		})
	}
}

func TestHTTPGatherer_Gather_Success(t *testing.T) {
	testData := "Hello from test server!"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testData))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	g := NewHTTPGatherer()

	tempDir := t.TempDir()
	dest := filepath.Join(tempDir, "downloaded_file.txt")

	ctx := context.Background()
	meta, err := g.Gather(ctx, server.URL+"/subdir/file.txt", dest)
	if err != nil {
		t.Fatalf("Gather returned unexpected error: %v", err)
	}

	fileContent, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(fileContent) != testData {
		t.Errorf("expected content %q, got %q", testData, string(fileContent))
	}

	httpMeta, ok := meta.(HTTPMetadata)
	if !ok {
		t.Fatalf("expected *HTTPMetadata, got %T", meta)
	}
	if httpMeta.URI != server.URL+"/subdir/file.txt" {
		t.Errorf("expected URI=%s, got %s", server.URL+"/subdir/file.txt", httpMeta.URI)
	}
	if httpMeta.Path != dest {
		t.Errorf("expected Path=%s, got %s", dest, httpMeta.Path)
	}
	if httpMeta.ResponseCode != http.StatusOK {
		t.Errorf("expected 200, got %d", httpMeta.ResponseCode)
	}
	if httpMeta.Size != int64(len(testData)) {
		t.Errorf("expected size=%d, got %d", len(testData), httpMeta.Size)
	}
	if httpMeta.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestHTTPGatherer_DefaultMaxResponseBytes(t *testing.T) {
	g := NewHTTPGatherer()
	if g.MaxResponseBytes != DefaultMaxResponseBytes {
		t.Errorf("expected default MaxResponseBytes=%d, got %d", DefaultMaxResponseBytes, g.MaxResponseBytes)
	}
}

func TestHTTPGatherer_Gather_ResponseBodyLimit(t *testing.T) {
	body := strings.Repeat("A", 1024) // 1 KiB
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	// Limit smaller than the body — the download must be refused.
	g := NewHTTPGatherer(WithMaxResponseBytes(512))
	dest := filepath.Join(t.TempDir(), "file.txt")

	_, err := g.Gather(context.Background(), server.URL+"/file.txt", dest)
	if err == nil {
		t.Fatalf("expected error when response body exceeds limit, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size-limit error, got %v", err)
	}
}

func TestHTTPGatherer_Gather_ResponseBodyLimit_PreservesExistingDest(t *testing.T) {
	body := strings.Repeat("A", 1024)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "file.txt")
	const original = "ORIGINAL CONTENT"
	if err := os.WriteFile(dest, []byte(original), 0600); err != nil {
		t.Fatalf("failed to seed destination: %v", err)
	}

	g := NewHTTPGatherer(WithMaxResponseBytes(512))
	_, err := g.Gather(context.Background(), server.URL+"/file.txt", dest)
	if err == nil {
		t.Fatalf("expected error when response body exceeds limit, got nil")
	}

	got, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatalf("destination should still exist: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("destination was modified on a rejected download: got %q, want %q", got, original)
	}
}

func TestHTTPGatherer_Gather_WithinResponseBodyLimit(t *testing.T) {
	body := strings.Repeat("A", 1024) // 1 KiB
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	g := NewHTTPGatherer(WithMaxResponseBytes(4096))
	dest := filepath.Join(t.TempDir(), "file.txt")

	meta, err := g.Gather(context.Background(), server.URL+"/file.txt", dest)
	if err != nil {
		t.Fatalf("unexpected error within limit: %v", err)
	}
	if got := meta.(HTTPMetadata).Size; got != int64(len(body)) {
		t.Errorf("expected size=%d, got %d", len(body), got)
	}
}

func TestHTTPGatherer_Gather_NoScheme(t *testing.T) {
	g := NewHTTPGatherer()
	ctx := context.Background()

	tempDir := t.TempDir()
	dest := filepath.Join(tempDir, "file.txt")

	_, err := g.Gather(ctx, "example.com/file.txt", dest)
	if err == nil {
		t.Fatal("expected an error when no scheme is provided, got nil")
	}
	if !strings.Contains(err.Error(), "no source scheme provided") {
		t.Errorf("expected error mentioning missing scheme, got %v", err)
	}
}

func TestHTTPGatherer_Gather_NoPath(t *testing.T) {
	g := NewHTTPGatherer()
	ctx := context.Background()

	tempDir := t.TempDir()
	dest := filepath.Join(tempDir, "file.txt")

	// Provide a URL with a scheme but no path
	_, err := g.Gather(ctx, "http://example.com", dest)
	if err == nil {
		t.Fatal("expected error when URL has no path, got nil")
	}
	if !strings.Contains(err.Error(), "specify a path to a file to download") {
		t.Errorf("expected error about specifying a path, got %v", err)
	}
}

func TestHTTPGatherer_Gather_Non200(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	g := NewHTTPGatherer()
	tempDir := t.TempDir()
	dest := filepath.Join(tempDir, "file.txt")

	ctx := context.Background()
	_, err := g.Gather(ctx, server.URL+"/missing-file.txt", dest)
	if err == nil {
		t.Fatal("expected an error for non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "received non-200 response code") {
		t.Errorf("expected error about non-200 response, got %v", err)
	}
}

func TestHTTPGatherer_Gather_EmptyDirDestination(t *testing.T) {
	testData := "Test data"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testData))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	g := NewHTTPGatherer()

	tempDir := t.TempDir()

	dest := filepath.Join(tempDir, "someDir") + "/"

	ctx := context.Background()
	srcURL := server.URL + "/download-me.bin"
	meta, err := g.Gather(ctx, srcURL, dest)
	if err != nil {
		t.Fatalf("Gather returned error: %v", err)
	}

	httpMeta := meta.(HTTPMetadata)
	expectedPath := filepath.Join(dest, "download-me.bin")
	if httpMeta.Path != expectedPath {
		t.Errorf("expected path=%s, got %s", expectedPath, httpMeta.Path)
	}

	fileContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(fileContent) != testData {
		t.Errorf("expected content=%q, got %q", testData, string(fileContent))
	}
}

func TestHTTPGatherer_Gather_CanceledContext(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow response so we can cancel the context
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte("Large amount of data..."))
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	g := NewHTTPGatherer()
	tempDir := t.TempDir()
	dest := filepath.Join(tempDir, "file.txt")

	// Create a context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := g.Gather(ctx, server.URL+"/slow-file", dest)
	if err == nil {
		t.Fatal("expected an error due to context cancellation, got nil")
	}
	if ctx.Err() == nil {
		t.Errorf("expected context to be canceled, got nil")
	}
}
