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

package file

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/enterprise-contract/go-gather/expand"
	"github.com/enterprise-contract/go-gather/gather"
	"github.com/enterprise-contract/go-gather/internal/helpers"
	"github.com/enterprise-contract/go-gather/metadata"
)

type FileGatherer struct {
	FSMetadata
}

type FSMetadata struct {
	Path      string
	Size      int64
	Timestamp string
}

type FileSaver struct {
	FSMetadata
}

func (f *FileGatherer) Matcher(uri string) bool {
	prefixes := []string{"file://", "file::", "/", "./", "../"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(uri, prefix) {
			return true
		}
	}
	return false
}

func (f *FileGatherer) Gather(ctx context.Context, src, dst string) (metadata.Metadata, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	for _, prefix := range []string{"file://", "file::"} {
		src = strings.TrimPrefix(src, prefix)
		dst = strings.TrimPrefix(dst, prefix)
	}
	src, err := helpers.ExpandPath(src)
	if err != nil {
		return nil, fmt.Errorf("failed to expand source path: %w", err)
	}
	dst, err = helpers.ExpandPath(dst)
	if err != nil {
		return nil, fmt.Errorf("failed to expand destination path: %w", err)
	}

	sInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source file does not exist: %w", err)
		}
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	if sInfo.IsDir() {
		if err := helpers.CopyDir(src, dst); err != nil {
			return nil, fmt.Errorf("failed to copy directory: %w", err)
		}
		dirSize, err := helpers.GetDirectorySize(dst)
		if err != nil {
			return nil, err
		}
		f.Path = dst
		f.Size = dirSize
		f.Timestamp = time.Now().String()
		return &f.FSMetadata, nil
	}

	compressedFile, err := expand.IsCompressedFile(src)
	if err != nil {
		return nil, err
	}
	if compressedFile {
		e, err := getExpander(src)
		if err != nil {
			return nil, err
		}
		err = e.Expand(ctx, src, dst, 0755)
		if err != nil {
			return nil, err
		}
		dirSize, err := helpers.GetDirectorySize(dst)
		if err != nil {
			return nil, err
		}
		f.Path = dst
		f.Size = dirSize
		f.Timestamp = time.Now().String()
		return &f.FSMetadata, nil
	}

	// TODO: Figure out how to make this flexible for different types of destinations
	fsaver := FileSaver{}
	return fsaver.save(ctx, src, dst, false)
}

func (f *FSMetadata) Get() interface{} {
	return f
}

func (f FSMetadata) GetPinnedURL(u string) (string, error) {
	if len(u) == 0 {
		return "", fmt.Errorf("empty file path")
	}
	for _, scheme := range []string{"file::", "file://"} {
		u = strings.TrimPrefix(u, scheme)
	}
	return "file::" + u, nil
}

// save copies from a filesystem source to a filesystem destination.
// If append is true, the file will be appended to the destination.
func (f *FileSaver) save(ctx context.Context, source string, destination string, append bool) (metadata.Metadata, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var dstFile *os.File
	var err error

	src, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source URI: %w", err)
	}

	if _, err := os.Stat(src.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("source file does not exist: %w", err)
	}

	dst, err := url.Parse(destination)
	if err != nil {
		return nil, fmt.Errorf("failed to parse destination URI: %w", err)
	}

	if append {
		dstFile, err = os.OpenFile(dst.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	} else {
		dstFile, err = os.Create(dst.Path)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dstFile.Close()

	srcFile, err := os.Open(src.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	writtenSize, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return nil, fmt.Errorf("failed to write to file: %w", err)
	}
	f.Path = dst.Path
	f.Size = writtenSize
	f.Timestamp = time.Now().Format(time.RFC3339)

	return &f.FSMetadata, nil
}

func getExpander(src string) (expand.Expander, error) {
	orderedFormats := []struct {
		format     string
		extensions []string
	}{
		{"tar", []string{"tar", "tar.gz", "tgz", "tar.bz2", "tbz2"}},
		{"bzip2", []string{"bz2"}},
		{"gzip", []string{"gzip", "gz"}},
		{"zip", []string{"zip"}},
	}

	for _, entry := range orderedFormats {
		for _, extension := range entry.extensions {
			if strings.HasSuffix(src, "."+extension) {
				return expand.GetExpander(entry.format), nil
			}
		}
	}
	return nil, fmt.Errorf("compressed file found, but no expander available")
}

func init() {
	gather.RegisterGatherer(&FileGatherer{})
}
