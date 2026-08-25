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

// Package http implements a Gatherer for HTTP and HTTPS sources.
package http

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/conforma/go-gather/gather"
	"github.com/conforma/go-gather/internal/helpers"
	"github.com/conforma/go-gather/metadata"
)

// DefaultMaxResponseBytes bounds the size of a downloaded response body,
// preventing a malicious or misbehaving server from exhausting disk. It matches
// the Conforma CLI's MaxRequestBodySize (80 MiB).
const DefaultMaxResponseBytes int64 = 80 << 20

// Option configures an HTTPGatherer.
type Option func(*HTTPGatherer)

// WithTransport sets the http.RoundTripper used by the HTTPGatherer's client.
func WithTransport(t http.RoundTripper) Option {
	return func(g *HTTPGatherer) { g.Client.Transport = t }
}

// WithMaxResponseBytes sets the maximum number of bytes read from a response
// body. A value <= 0 disables the limit.
func WithMaxResponseBytes(n int64) Option {
	return func(g *HTTPGatherer) { g.MaxResponseBytes = n }
}

// HTTPGatherer gathers resources over HTTP/HTTPS.
type HTTPGatherer struct {
	Client http.Client
	// MaxResponseBytes bounds the response body size. A value <= 0 disables the
	// limit. NewHTTPGatherer sets it to DefaultMaxResponseBytes.
	MaxResponseBytes int64
}

// HTTPMetadata holds metadata about a gathered HTTP resource.
type HTTPMetadata struct {
	URI          string
	Path         string
	ResponseCode int
	Size         int64
	Timestamp    string
}

// NewHTTPGatherer returns an HTTPGatherer with a default 30-second timeout.
func NewHTTPGatherer(opts ...Option) *HTTPGatherer {
	g := &HTTPGatherer{
		Client:           http.Client{Timeout: 30 * time.Second},
		MaxResponseBytes: DefaultMaxResponseBytes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	return g
}

// Gather downloads a file from rawSource via HTTP and writes it to dst.
func (h *HTTPGatherer) Gather(ctx context.Context, rawSource, dst string) (meta metadata.Metadata, err error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	src, err := url.Parse(rawSource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source URI: %w", err)
	}

	// Check if the source scheme is provided
	if src.Scheme == "" {
		return nil, fmt.Errorf("no source scheme provided")
	}

	// Check if the source filename is provided
	if src.Path == "" {
		return nil, fmt.Errorf("specify a path to a file to download")
	}

	// Get the source filename
	sourceFileName := filepath.Base(src.Path)

	// Expand the destination path
	dst, err = helpers.ExpandPath(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to expand destination path: %w", err)
	}

	// Check if the destination has a trailing slash.
	// If it does, append the source filename to the destination path.
	if strings.HasSuffix(dst, "/") {
		dst = filepath.Join(dst, sourceFileName)
	} else {
		// If it doesn't, append the source filename to the destination path.
		if filepath.Ext(dst) == "" {
			dst = filepath.Join(dst, "/", sourceFileName)
		}
	}

	// Create a new HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", rawSource, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set the User-Agent header
	req.Header.Set("User-Agent", "Go-Gather")

	// Perform the HTTP request
	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download from URL: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response code is "ok"
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	// Create the destination directory.
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Stream into a temporary file in the destination directory and only rename
	// it into place after the size is validated, so a rejected or failed
	// download never truncates or partially overwrites an existing destination.
	tmpFile, err := createDownloadTemp(filepath.Dir(dst))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file in %q: %w", filepath.Dir(dst), err)
	}
	tmpName := tmpFile.Name()
	committed := false
	defer func() {
		tmpFile.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	// Bound the response body so a malicious or misbehaving server cannot
	// exhaust the destination filesystem. Reading up to limit+1 bytes lets us
	// detect a body that exceeds the limit rather than silently truncating it,
	// which io.LimitReader would otherwise do without any signal.
	reader := io.Reader(resp.Body)
	if h.MaxResponseBytes > 0 {
		if resp.ContentLength > h.MaxResponseBytes {
			return nil, fmt.Errorf("response body size %d exceeds limit of %d bytes", resp.ContentLength, h.MaxResponseBytes)
		}
		// math.MaxInt64 means "effectively unlimited"; adding one would overflow
		// to a negative LimitReader bound and read nothing, so skip the wrap.
		if h.MaxResponseBytes < math.MaxInt64 {
			reader = io.LimitReader(resp.Body, h.MaxResponseBytes+1)
		}
	}

	bytesWritten, err := io.Copy(tmpFile, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write to destination file: %w", err)
	}
	if h.MaxResponseBytes > 0 && bytesWritten > h.MaxResponseBytes {
		return nil, fmt.Errorf("response body exceeds limit of %d bytes", h.MaxResponseBytes)
	}

	// If the destination already exists, preserve its mode. A new file keeps the
	// umask-respecting mode it was created with (matching os.Create), so this
	// neither forces downloads owner-only nor makes them more permissive than the
	// caller's umask allows.
	if fi, statErr := os.Stat(dst); statErr == nil {
		if err = os.Chmod(tmpName, fi.Mode().Perm()); err != nil {
			return nil, fmt.Errorf("failed to set destination file mode: %w", err)
		}
	}

	// Flush and atomically move the validated file into place. This intentionally
	// *replaces* dst rather than reproducing os.Create's behavior of following a
	// symlink at dst and writing through it, which is a write-through-symlink
	// TOCTOU risk. An existing regular file's permission bits are preserved above;
	// setuid/setgid are deliberately not carried onto a freshly downloaded file.
	if err = tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err = os.Rename(tmpName, dst); err != nil {
		return nil, fmt.Errorf("failed to move downloaded file into place: %w", err)
	}
	committed = true

	return HTTPMetadata{
		URI:          rawSource,
		Path:         dst,
		ResponseCode: resp.StatusCode,
		Size:         bytesWritten,
		Timestamp:    time.Now().Format(time.RFC3339),
	}, nil
}

// createDownloadTemp creates a uniquely-named temporary file in dir whose
// permissions honor the process umask, matching os.Create. os.CreateTemp forces
// mode 0600, which would make every download owner-only regardless of umask; we
// reuse its unique name but recreate the file with 0666 so the umask applies.
func createDownloadTemp(dir string) (*os.File, error) {
	f, err := os.CreateTemp(dir, ".gather-*.tmp")
	if err != nil {
		return nil, err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return nil, err
	}
	if err := os.Remove(name); err != nil {
		return nil, err
	}
	// O_EXCL fails closed if anything (re)created the path in the meantime.
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666) //#nosec G302 -- umask applies at creation; intentional os.Create parity
}

// Matcher returns true if the URI uses an HTTP or HTTPS scheme and is not a known git host.
func (h *HTTPGatherer) Matcher(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, vendor := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		if host == vendor || strings.HasSuffix(host, "."+vendor) {
			return false
		}
	}
	return true
}

// Get returns the HTTPMetadata value.
func (h HTTPMetadata) Get() interface{} {
	return h
}

// GetPinnedURL returns an http:: prefixed URL for the given address.
func (h HTTPMetadata) GetPinnedURL(u string) (string, error) {
	if len(u) == 0 {
		return "", fmt.Errorf("empty URL")
	}
	for _, scheme := range []string{"http://", "https://", "http::"} {
		u = strings.TrimPrefix(u, scheme)
	}
	return "http::" + u, nil
}

func init() {
	gather.RegisterGatherer(NewHTTPGatherer())
}
