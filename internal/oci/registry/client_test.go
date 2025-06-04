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

package registry

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

func TestRepositoryPlainHTTP(t *testing.T) {
	no := false
	yes := true
	cases := []struct {
		name      string
		flag      *bool
		registry  string
		plainHTTP bool
	}{
		{
			name:      "tls=false",
			flag:      &no,
			plainHTTP: true,
		},
		{
			name:      "tls=true",
			flag:      &yes,
			plainHTTP: false,
		},
		{
			name:      "hostname=localhost:5000",
			registry:  "localhost:5000",
			plainHTTP: true,
		},
		{
			name:      "hostname=[::1]:5000",
			registry:  "[::1]:5000",
			plainHTTP: true,
		},
		{
			name:      "hostname=localhost:5000,tls=true",
			flag:      &yes,
			registry:  "localhost:5000",
			plainHTTP: false,
		},
		{
			name:      "hostname=[::1]:5000,tls=true",
			flag:      &yes,
			registry:  "[::1]:5000",
			plainHTTP: false,
		},
		{
			name:      "hostname=quay.io",
			registry:  "quay.io",
			plainHTTP: false,
		},
		{
			name:      "hostname=quay.io,tls=false",
			flag:      &no,
			registry:  "quay.io",
			plainHTTP: true,
		},
		{
			name:      "hostname=quay.io,tls=false",
			flag:      &yes,
			registry:  "quay.io",
			plainHTTP: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := remote.Repository{}

			if c.registry != "" {
				r.Reference = registry.Reference{Registry: c.registry}
			}

			viper.AutomaticEnv()
			viper.SetEnvPrefix("TEST")
			if c.flag != nil {
				t.Setenv("TEST_TLS", strconv.FormatBool(*c.flag))
			}

			err := SetupClient(&r, nil)
			if err != nil {
				t.Fatal(err)
			}

			if expected, got := c.plainHTTP, r.PlainHTTP; expected != got {
				t.Errorf("PlainHTTP = %v, expected %v", got, expected)
			}
		})
	}
}

func getClientTransport(r *remote.Repository) remote.Client {
	return r.Client
}

func TestOCIGatherer_Gather_SetClientTransport(t *testing.T) {
	r := &remote.Repository{}

	customTransport := &http.Transport{
		MaxIdleConns:    100,
	}
	// Set a custom transport
	SetupClient(r, customTransport)
	client := getClientTransport(r)
	fmt.Printf("\n\nrepository: %v\n\n\n", reflect.TypeOf(client))
	//ensure that we have the same transport
	assert.Equal(t, client, customTransport, fmt.Sprintf("expected Transport to be, got %v", reflect.TypeOf(customTransport)))
}
