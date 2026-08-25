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
#   check-commit-message.sh [--strict] <file>   # read the message from a file
#   ... | check-commit-message.sh [--strict]    # or read it from stdin
#
# Modes:
#   default   Lenient, for the local commit-msg hook operating on git's commit
#             buffer: leading '#' comment lines are ignored and autosquash
#             (fixup!/squash!/amend!) commits are allowed, since they are meant
#             to be squashed away before the branch is pushed.
#   --strict  For CI, validating an already-created commit's real subject:
#             '#' lines are significant (a subject may legitimately start with
#             '#') and autosquash commits are rejected, because merge-based
#             integration preserves every commit on main.
#
# Exit status: 0 if the message is valid or exempt, 1 otherwise.
set -uo pipefail

# Allowed types, matching the conventionalcommits preset used in .releaserc.
readonly TYPES='feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert'
readonly HEADER_RE="^(${TYPES})(\([^)]+\))?!?: .+"

strict=false
file=""
for arg in "$@"; do
  case "$arg" in
    --strict) strict=true ;;
    *) file="$arg" ;;
  esac
done

# Read the raw message from the file argument if present, otherwise from stdin.
if [[ -n "$file" && -f "$file" ]]; then
  raw="$(cat -- "$file")"
else
  raw="$(cat)"
fi

if [[ "$strict" == true ]]; then
  # An already-created commit: its subject is the first line, verbatim.
  header="$(printf '%s\n' "$raw" | head -n1)"
else
  # git's commit buffer: the header is the first non-empty, non-comment line.
  # Comment lines (starting with '#') are appended by git and must be ignored.
  header=""
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    [[ "$line" == \#* ]] && continue
    header="$line"
    break
  done <<< "$raw"
fi

# Exempt machine-generated messages that legitimately land on main: merge
# commits and git's default revert messages (the conventionalcommits parser
# recognizes 'Revert "..."').
case "$header" in
  "Merge "*|"Revert "*)
    exit 0
    ;;
esac

# Autosquash commits are a local convenience only; reject them under --strict so
# they cannot reach main unsquashed.
if [[ "$strict" != true ]]; then
  case "$header" in
    "fixup! "*|"squash! "*|"amend! "*)
      exit 0
      ;;
  esac
fi

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
