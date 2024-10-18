module github.com/enterprise-contract/go-gather/gather/file

go 1.22.5

require (
	github.com/enterprise-contract/go-gather v0.0.3
	github.com/enterprise-contract/go-gather/expander v0.0.1
	github.com/enterprise-contract/go-gather/metadata v0.0.3-0.20241015082844-9df651247f12
	github.com/enterprise-contract/go-gather/metadata/file v0.0.2-0.20241015082844-9df651247f12
	github.com/enterprise-contract/go-gather/saver v0.0.2
)

require github.com/enterprise-contract/go-gather/saver/file v0.0.1 // indirect

replace (
	github.com/enterprise-contract/go-gather => ../..
	github.com/enterprise-contract/go-gather/expander => ../../expander
	github.com/enterprise-contract/go-gather/metadata => ../../metadata
	github.com/enterprise-contract/go-gather/metadata/file => ../../metadata/file
	github.com/enterprise-contract/go-gather/saver => ../../saver
)
