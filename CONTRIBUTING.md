# Contributing to go-gather

## Commit conventions

go-gather is released automatically with
[semantic-release](https://semantic-release.gitbook.io/) using the
[Conventional Commits](https://www.conventionalcommits.org/) preset (see
`.releaserc`). Because pull requests are merged with merge commits, **every
individual commit** that lands on `main` must use a conventional message —
the commit type is what determines version bumps and release notes.

Commit messages must follow:

```
<type>[optional scope][!]: <description>
```

Allowed types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
`build`, `ci`, `chore`, `revert`. A `!` before the colon (or a
`BREAKING CHANGE:` footer) marks a breaking change.

Examples:

```
feat(oci): add retry to default transport
fix: guard option constructors against nil options
docs: add godoc comments to exported symbols
refactor(http)!: change SetupClient signature
```

### Enforcement

- **Locally (opt-in):** run `make install-hooks` once. This points
  `core.hooksPath` at `.githooks`, enabling a `commit-msg` hook that rejects
  non-conventional messages before the commit is created.
- **In CI:** the `Commit Lint` workflow validates every commit in a pull
  request and must pass before merge.

Both use the same validator, `hack/check-commit-message.sh`, so local and CI
results agree. Run its tests with `make test-hooks`.
