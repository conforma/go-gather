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

// Package git implements a Gatherer for git repository sources.
package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	giturls "github.com/chainguard-dev/git-urls"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/conforma/go-gather/gather"
	"github.com/conforma/go-gather/internal/helpers"
	"github.com/conforma/go-gather/metadata"
)

var filePathPattern = regexp.MustCompile(`^(\./|\../|/|[a-zA-Z]:\\|~\/).*`)

// GitGatherer gathers resources by cloning git repositories.
type GitGatherer struct {
	GitMetadata
	Authenticator SSHAuthenticator
}

// GitMetadata holds metadata about a gathered git repository.
type GitMetadata struct {
	Path         string
	CommitHash   string
	Author       string
	Timestamp    string
	LatestCommit string
}

// SSHAuthenticator represents an interface for authenticating SSH connections.
type SSHAuthenticator interface {
	// NewSSHAgentAuth returns a new SSH agent authentication method for the given user.
	// It returns an instance of transport.AuthMethod and an error if any.
	NewSSHAgentAuth(user string) (transport.AuthMethod, error)
}

// RealSSHAuthenticator represents an implementation of the SSHAuthenticator interface.
type RealSSHAuthenticator struct{}

// Matcher returns true if the URI looks like a git repository.
func (g *GitGatherer) Matcher(uri string) bool {
	prefixes := []string{"git@", "git://", "git::"}
	for _, term := range prefixes {
		if strings.HasPrefix(uri, term) {
			return true
		}
	}

	// check if we have any magic prefixes (::) in the URL
	// If we do, we don't want to match it as a git URL.
	if strings.Contains(uri, "::") {
		return false
	}

	for _, term := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		if strings.Contains(uri, term) {
			return true
		}
	}

	return strings.HasSuffix(uri, ".git")
}

// Gather clones a git repository from src into dst.
func (g *GitGatherer) Gather(ctx context.Context, src, dst string) (metadata.Metadata, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	// Process our provided source URL to get the source URL, ref, subdir, and depth
	src, ref, subdir, depth, err := processUrl(src)
	if err != nil {
		return nil, fmt.Errorf("failed to process URL: %w", err)
	}

	// Initialize the clone options for the git repository
	cloneOpts := &git.CloneOptions{
		URL:             src,
		InsecureSkipTLS: os.Getenv("GIT_SSL_NO_VERIFY") == "true",
	}

	// If we have a ref and it isn't a hash, set the reference name in the clone options
	if len(ref) > 0 && !plumbing.IsHash(ref) {
		cloneOpts.ReferenceName = plumbing.ReferenceName(ref)
	}

	if depth != "" {
		cloneOpts.Depth, err = strconv.Atoi(depth)
		if err != nil {
			return nil, fmt.Errorf("failed to parse depth: %w", err)
		}
	}

	// Initialize the git repository and worktree
	r := &git.Repository{}
	w := &git.Worktree{}

	// tmpDir is used to clone the repository if a subdir is specified
	var tmpDir string

	if subdir != "" {
		tmpDir, err = os.MkdirTemp("", "git-repo-")
		if err != nil {
			return nil, fmt.Errorf("error creating temporary directory: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		r, err = git.PlainCloneContext(ctx, tmpDir, false, cloneOpts)
		if err != nil {
			return nil, fmt.Errorf("error cloning repository: %w", err)
		}
	} else {
		r, err = git.PlainCloneContext(ctx, dst, false, cloneOpts)
		if err != nil {
			return nil, fmt.Errorf("error cloning repository: %w", err)
		}
	}

	if ref != "" {
		h, err := r.ResolveRevision(plumbing.Revision(ref))
		if err != nil {
			return nil, fmt.Errorf("error resolving ref: %w", err)
		}
		w, err = r.Worktree()
		if err != nil {
			return nil, fmt.Errorf("error getting worktree: %w", err)
		}
		checkoutOpts := &git.CheckoutOptions{
			Hash: *h,
		}
		err = w.Checkout(checkoutOpts)
		if err != nil {
			return nil, fmt.Errorf("error checking out ref: %w", err)
		}
	}

	if subdir != "" {
		path := filepath.Join(tmpDir, subdir)
		if !helpers.IsPathContained(path, tmpDir) {
			return nil, fmt.Errorf("subdir %q traverses outside the repository root", subdir)
		}
		w, err = r.Worktree()
		if err != nil {
			return nil, fmt.Errorf("error getting worktree: %w", err)
		}
		_, err = w.Filesystem.Stat(subdir)
		if err != nil {
			return nil, fmt.Errorf("path %s does not exist in the repository", subdir)
		}
		// Copy the subdir through an os.Root anchored at the repository root so
		// that a malicious repository cannot use an in-tree symlink to escape
		// tmpDir and exfiltrate host files. The lexical check above remains a
		// cheap early reject for '..' traversal.
		err = helpers.CopyDirConfined(tmpDir, subdir, dst)
		if err != nil {
			return nil, fmt.Errorf("error copying directory: %w", err)
		}
	}

	head, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("determining the HEAD reference: %w", err)
	}

	m := GitMetadata{
		LatestCommit: head.Hash().String(),
	}
	return &m, nil
}

// Get returns the GitMetadata value.
func (g GitMetadata) Get() interface{} {
	return g
}

// GetPinnedURL returns a git:: URL pinned to the latest commit hash.
func (g GitMetadata) GetPinnedURL(u string) (string, error) {
	if len(u) == 0 {
		return "", fmt.Errorf("empty URL")
	}
	if g.LatestCommit == "" {
		return "", fmt.Errorf("latest commit not set")
	}
	for _, scheme := range []string{"git::", "git://", "https://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	if strings.HasPrefix(u, "git@") {
		u = strings.Replace(strings.Split(u, "git@")[1], ":", "/", 1)
	}
	return "git::" + strings.SplitN(u, "?ref=", 2)[0] + "?ref=" + g.LatestCommit, nil
}

// NewSSHAgentAuth returns an AuthMethod that uses the SSH agent for authentication.
// It uses the specified user as the username for authentication.
func (r *RealSSHAuthenticator) NewSSHAgentAuth(user string) (transport.AuthMethod, error) {
	return ssh.NewSSHAgentAuth(user)
}

// extractKeyFromQuery extracts the value of the specified key from the query parameters and extracts a subdir, if present.
func extractKeyFromQuery(q url.Values, key string, subdir *string) string {
	value := q.Get(key)
	if strings.Contains(value, "//") {
		parts := strings.SplitN(value, "//", 2)
		*subdir = parts[1]
		q.Del(key)
		return parts[0]
	}
	q.Del(key)
	return value
}

func processUrl(rawSource string) (src, ref, subdir, depth string, err error) {
	// Convert SCP-style git@host:path to HTTPS before generic prefix stripping,
	// since stripping git@ leaves host:path which confuses URL parsers when
	// https:// is prepended (the colon is misinterpreted as a port separator).
	if strings.HasPrefix(rawSource, "git@") {
		rawSource = strings.TrimPrefix(rawSource, "git@")
		rawSource = strings.Replace(rawSource, ":", "/", 1)
		rawSource = "https://" + rawSource
	}

	// Remove any prefixes we normally see from the source URL.
	terms := []string{"git://", "git::", "https://", "file://", "file::"}
	for _, prefix := range terms {
		rawSource = strings.TrimPrefix(rawSource, prefix)
	}
	src = rawSource

	if filePathPattern.MatchString(src) {
		src = "file://" + src
	}

	if !strings.HasPrefix(src, "file://") {
		src = "https://" + src
	}

	parsedUrl, err := giturls.Parse(src)
	if err != nil {
		return src, ref, subdir, depth, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Parse the URL again with the url package to extract the query parameters, etc.
	u, err := url.Parse(parsedUrl.String())
	if err != nil {
		return src, ref, subdir, depth, fmt.Errorf("failed to reparse URL: %w", err)
	}

	// Extract the ref, subdir, and depth from the query parameters
	q := u.Query()
	ref = extractKeyFromQuery(q, "ref", &subdir)
	depth = extractKeyFromQuery(q, "depth", &subdir)
	u.RawQuery = q.Encode()

	// If the path contains "//", split it to get the actual path and subdir
	if strings.Contains(u.Path, "//") {
		parts := strings.SplitN(u.Path, "//", 2)
		u.Path = parts[0]
		subdir = parts[1]
	}

	// If the path does not end with ".git", append it
	if !strings.HasSuffix(u.Path, ".git") && !strings.HasPrefix(src, "file://") {
		u.Path += ".git"
	}

	// Return the URL, ref, subdir, and depth
	return u.String(), ref, subdir, depth, nil
}

func init() {
	gather.RegisterGatherer(&GitGatherer{})
}
