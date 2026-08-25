MAKEFLAGS+=-j --no-print-directory
_SHELL := bash
SHELL=$(if $@,$(info ? [1m$@[0m))$(_SHELL)
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
COPY:=The Enterprise Contract Contributors

##@ Information

.PHONY: help
help: ## Display this help
	@awk 'function ww(s) {\
		if (length(s) < 59) {\
			return s;\
		}\
		else {\
			r="";\
			l="";\
			split(s, arr, " ");\
			for (w in arr) {\
				if (length(l " " arr[w]) > 59) {\
					r=r l "\n                     ";\
					l="";\
				}\
				l=l " " arr[w];\
			}\
			r=r l;\
			return r;\
		}\
	} BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9%/_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", "make " $$1, ww($$2) } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

# The go.work file is git ignored and not checked in. Create one if
# it doesn't exist already. (Note we're using absolute paths.)
go.work:
	ROOT=$$(git rev-parse --show-toplevel) && \
		cd $${ROOT} && \
		go work init && \
		go work use -r $${ROOT}

.PHONY: test
test: go.work ## Run all unit tests
	@echo "Unit tests:"
	@go test -race -covermode=atomic -coverprofile=coverage-unit.out -timeout 30s -tags=unit $$(go work edit -json | jq -c -r '[.Use[].DiskPath] | map_values(. + "/...")[]')

.PHONY: lint
lint: go.work go-mod-lint ## Run linter
	@golangci-lint run -- $$(go work edit -json | jq -c -r '[.Use[].DiskPath] | map_values(. + "/...")[]')

.PHONY: go-mod-lint
go-mod-lint:
	@echo "Scanning for go.mod files and performing tidy..."
	@find . -name "go.mod" -execdir go mod tidy >/dev/null 2>&1 \;
	@echo "Checking for modified go.mod or go.sum files..."
	@if git status --porcelain | grep -q -e "go.mod" -e "go.sum"; then \
		echo "Ensure the following go.mod or go.sum files are added to the git commit:"; \
		git status --porcelain | grep -e "go.mod" -e "go.sum"; \
	else \
		echo "No go.mod or go.sum files need to be added to the git commit."; \
	fi

.PHONY: lint-fix
lint-fix: go.work
	@golangci-lint run --fix -- $$(go work edit -json | jq -c -r '[.Use[].DiskPath] | map_values(. + "/...")[]')

##@ Git hooks

.PHONY: install-hooks
install-hooks: ## Enable the repo's git hooks (Conventional Commits check)
	@git config core.hooksPath .githooks
	@echo "Git hooks enabled: core.hooksPath -> .githooks"

.PHONY: test-hooks
test-hooks: ## Run the commit-message validator's tests
	@bash hack/check-commit-message_test.sh
