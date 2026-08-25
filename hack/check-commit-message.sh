#!/usr/bin/env bash
# Validate a commit message against the Conventional Commits format.
#
# go-gather releases with semantic-release using the "conventionalcommits"
# preset (see .releaserc), so every commit that lands on main must carry a
# conventional type prefix or it contributes nothing to version bumps and
# release notes. This is the single source of truth shared by the local
# commit-msg hook (.githooks/commit-msg) and the CI check
# (.github/workflows/commit-lint.yaml).
#
# Usage:
#   check-commit-message.sh <file>   # read the message from a file
#   ... | check-commit-message.sh    # or read the message from stdin
#
# Exit status: 0 if the message is valid or exempt, 1 otherwise.
set -uo pipefail

# Allowed types, matching the conventionalcommits preset used in .releaserc.
readonly TYPES='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert'
readonly HEADER_RE="^(${TYPES})(\([^)]+\))?!?: .+"

# Read the raw message from the file argument if present, otherwise from stdin.
if [[ $# -ge 1 && -f "$1" ]]; then
  raw="$(cat -- "$1")"
else
  raw="$(cat)"
fi

# The header is the first non-empty, non-comment line. Comment lines (starting
# with '#') are what git appends to the commit-msg buffer and must be ignored.
header=""
while IFS= read -r line; do
  [[ -z "$line" ]] && continue
  [[ "$line" == \#* ]] && continue
  header="$line"
  break
done <<< "$raw"

# Exempt machine-generated messages we do not author by hand: merge commits,
# git's default revert messages, and autosquash fixup/squash/amend commits.
case "$header" in
  "Merge "*|"Revert "*|"fixup! "*|"squash! "*|"amend! "*)
    exit 0
    ;;
esac

if [[ "$header" =~ $HEADER_RE ]]; then
  exit 0
fi

cat >&2 <<EOF
Invalid commit message header:

  ${header:-<empty>}

Commit messages must follow Conventional Commits:

  <type>[optional scope][!]: <description>

Allowed types: ${TYPES//|/, }

Examples:
  feat(oci): add retry to default transport
  fix: guard option constructors against nil options
  docs: add godoc comments to exported symbols
  refactor(http)!: change SetupClient signature   # ! marks a breaking change

See https://www.conventionalcommits.org and .releaserc for why this matters.
EOF
exit 1
