module github.com/enterprise-contract/go-gather/metadata/git

go 1.22.5

require (
	github.com/enterprise-contract/go-gather/metadata v0.0.2
	github.com/go-git/go-git/v5 v5.12.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/stretchr/testify v1.9.0
	gopkg.in/yaml.v3 v3.0.1 // indirect

)

replace github.com/enterprise-contract/go-gather/metadata => ../../metadata
