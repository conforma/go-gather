#!/usr/bin/env bash
# Tests for check-commit-message.sh
#
# Runs a table of commit-message fixtures through the validator and asserts the
# expected exit status. Run directly: hack/check-commit-message_test.sh
#
# The validator has two modes:
#   - default (local commit-msg hook): lenient. Strips leading '#' comment
#     lines from the git buffer and exempts autosquash (fixup!/squash!/amend!)
#     commits, which are expected to be squashed away before the branch is
#     pushed.
#   - --strict (CI): validates the stored commit's real subject. Does not strip
#     '#' lines and does not exempt autosquash commits, because merge-based
#     integration preserves every commit on main.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALIDATOR="${SCRIPT_DIR}/check-commit-message.sh"

pass=0
fail=0

# assert <expected-exit> <description> <message> [validator-args...]
# The message is passed on stdin so multi-line bodies are preserved.
assert() {
  local expected="$1" desc="$2" msg="$3"
  shift 3
  local got
  printf '%s' "$msg" | "${VALIDATOR}" "$@" >/dev/null 2>&1
  got=$?
  if [[ "$got" -eq "$expected" ]]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    printf 'FAIL: %s\n      expected exit %s, got %s\n' "$desc" "$expected" "$got"
  fi
}

# --- Valid conventional messages (exit 0), both modes ---
assert 0 "plain type"                 "feat: add thing"
assert 0 "type with scope"            "fix(oci): wrap default transport"
assert 0 "breaking with bang"         "feat!: drop deprecated API"
assert 0 "breaking with scope + bang" "refactor(http)!: change signature"
assert 0 "chore type"                 "chore: update deps"
assert 0 "docs type"                  "docs: add godoc comments"
assert 0 "ci type"                    "ci: pin action to sha"
assert 0 "header + body"              $'feat: add retry\n\nAdds exponential backoff to the client.'
assert 0 "strict: plain type"         "feat: add thing" --strict

# --- Exempt in both modes: machine-generated merge/revert ---
assert 0 "merge commit"              "Merge pull request #339 from robnester-rh/EC-2053"
assert 0 "git revert commit"         'Revert "feat: add retry"'
assert 0 "strict: merge commit"      "Merge pull request #339 from robnester-rh/EC-2053" --strict
assert 0 "strict: git revert commit" 'Revert "feat: add retry"' --strict

# --- Autosquash: exempt in hook mode, rejected in strict/CI mode ---
assert 0 "hook: fixup autosquash"    "fixup! feat: add retry"
assert 0 "hook: squash autosquash"   "squash! feat: add retry"
assert 0 "hook: amend autosquash"    "amend! feat: add retry"
assert 1 "strict: fixup rejected"    "fixup! feat: add retry" --strict
assert 1 "strict: squash rejected"   "squash! feat: add retry" --strict
assert 1 "strict: amend rejected"    "amend! feat: add retry" --strict

# --- Comment handling: stripped in hook mode, significant in strict/CI mode ---
assert 0 "hook: leading comment then valid subject" $'# please enter the commit message\nfeat: real subject'
assert 1 "strict: hash subject not skipped"         $'# invalid subject\n\nfeat: accepted body' --strict
assert 0 "strict: valid subject with body"          $'feat: real subject\n\n# a comment in the body' --strict

# --- Invalid messages (non-zero), both modes ---
assert 1 "no type prefix"        "Add resource limits to archive expanders"
assert 1 "unknown type"          "feature: add thing"
assert 1 "arbitrary word type"   "foo: bar"
assert 1 "missing colon"         "feat add thing"
assert 1 "missing space"         "feat:add thing"
assert 1 "empty subject"         "feat: "
assert 1 "empty message"         ""
assert 1 "strict: no type prefix" "Add resource limits to archive expanders" --strict

printf '\n%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]]
