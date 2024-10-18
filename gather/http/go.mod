module github.com/enterprise-contract/go-gather/gather/http

go 1.22.5

require (
	github.com/enterprise-contract/go-gather v0.0.3
	github.com/enterprise-contract/go-gather/metadata v0.0.2
	github.com/enterprise-contract/go-gather/metadata/http v0.0.1
	github.com/enterprise-contract/go-gather/saver v0.0.2
	github.com/stretchr/testify v1.9.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/enterprise-contract/go-gather/saver/file v0.0.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/enterprise-contract/go-gather => ../..
	github.com/enterprise-contract/go-gather/metadata => ../../metadata
	github.com/enterprise-contract/go-gather/metadata/http => ../../metadata/http
	github.com/enterprise-contract/go-gather/saver => ../../saver
)
