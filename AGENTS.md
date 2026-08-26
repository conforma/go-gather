# Agent instructions for go-gather

Guidance for LLM coding agents working in this repository. (Claude Code also
reads this file; it mirrors the human-facing `CONTRIBUTING.md`.)

## Commit messages

This repository releases automatically with semantic-release using the
[Conventional Commits](https://www.conventionalcommits.org/) preset (see
`.releaserc`). Pull requests are merged with merge commits, so **every
commit** you create must follow the format below or CI (`Commit Lint`) will
fail the pull request.

Format:

```text
<type>[optional scope][!]: <description>
```

- **type** (required): one of `feat`, `fix`, `docs`, `style`, `refactor`,
  `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- **scope** (optional): the affected area, e.g. `fix(oci): ...`.
- **`!`** (optional): before the colon marks a breaking change, e.g.
  `refactor(http)!: ...`. A `BREAKING CHANGE:` footer works too.
- **description**: imperative mood, lower-case, with a space after the colon.

Rules of thumb:

- Pick the type by intent: `feat` for new behavior (minor release), `fix` for
  bug fixes (patch release); `docs`, `style`, `refactor`, `perf`, `test`,
  `build`, `ci`, and `chore` for everything else.
- Do not put a Jira ticket key in the subject line. If a ticket applies, add a
  `Ref: EC-XXXX` trailer after any `Co-Authored-By` trailer.
- Keep the subject concise; wrap the body at ~72 columns.

Examples:

```text
feat(oci): add retry to default transport
fix: guard option constructors against nil options
docs: add godoc comments to exported symbols
ci: pin codecov-action to a commit SHA
refactor(http)!: change SetupClient signature
```

Before committing, run `make install-hooks` once to enable the local
`commit-msg` hook, which validates messages with the same script CI uses
(`hack/check-commit-message.sh`).
