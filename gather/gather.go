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

// Package gather provides functionality for downloading data from different sources.
// It defines the Gatherer interface and implements various gatherers for different protocols.
// The Gather function determines the protocol from the source protocol and uses the appropriate
// Gatherer to perform the operation. It returns metadata for the downloaded data and an error, if any.
package gather

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	gogather "github.com/enterprise-contract/go-gather"
	"github.com/enterprise-contract/go-gather/gather/file"
	"github.com/enterprise-contract/go-gather/gather/git"
	"github.com/enterprise-contract/go-gather/gather/http"
	"github.com/enterprise-contract/go-gather/gather/oci"
	"github.com/enterprise-contract/go-gather/metadata"
)

// Gatherer is an interface that defines the behavior of a gatherer.
type Gatherer interface {
	Gather(ctx context.Context, source, destination string) (metadata metadata.Metadata, err error)
}

// protocolHandlers maps URL schemes to their corresponding Gatherer implementations.
var protocolHandlers = map[string]Gatherer{
	"FileURI": &file.FileGatherer{},
	"GitURI":  &git.GitGatherer{},
	"HTTPURI": &http.HTTPGatherer{},
	"OCIURI":  &oci.OCIGatherer{},
}

// Gather determines the protocol from the source URI and uses the appropriate Gatherer to perform the operation.
// It returns the gathered metadata and an error, if any.
func Gather(ctx context.Context, unresolvedSource, destination string) (metadata.Metadata, error) {
	source, err := resolveSource(ctx, unresolvedSource)
	if err != nil {
		return nil, fmt.Errorf("resolving source %q: %w", unresolvedSource, err)
	}

	srcProtocol, err := gogather.ClassifyURI(source)
	if err != nil {
		return nil, fmt.Errorf("failed to classify source URI: %w", err)
	}

	if gatherer, ok := protocolHandlers[srcProtocol.String()]; ok {
		return gatherer.Gather(ctx, source, destination)
	}
	return nil, fmt.Errorf("unsupported source protocol: %s", srcProtocol)

}

func resolveSource(ctx context.Context, unresolved string) (string, error) {
	funcs := template.FuncMap{}
	for name, resolver := range resolvers {
		resolver := resolver
		funcs[name] = func(raw string) (string, error) {
			return resolver.Resolve(ctx, raw)
		}
	}

	t, err := template.New("go-gather").Funcs(funcs).Parse(unresolved)
	if err != nil {
		return "", fmt.Errorf("creating resolver template: %w", err)
	}
	var resolved bytes.Buffer
	if err := t.Execute(&resolved, nil); err != nil {
		return "", fmt.Errorf("executing resolver template: %w", err)
	}

	return resolved.String(), nil
}

type Resolver interface {
	Resolve(context.Context, string) (string, error)
}

var resolvers map[string]Resolver

func RegisterResolver(name string, resolver Resolver) error {
	if resolvers == nil {
		resolvers = map[string]Resolver{}
	}
	if _, found := resolvers[name]; found {
		return fmt.Errorf("resolver named %q already registered", name)
	}

	resolvers[name] = resolver
	return nil
}
