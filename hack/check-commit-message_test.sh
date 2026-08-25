#!/usr/bin/env bash
# Tests for check-commit-message.sh
#
# Runs a table of commit-message fixtures through the validator and asserts the
# expected exit status. Run directly: hack/check-commit-message_test.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALIDATOR="${SCRIPT_DIR}/check-commit-message.sh"

pass=0
fail=0

# assert <expected-exit> <description> <message...>
# The message is passed on stdin so multi-line bodies are preserved.
assert() {
  local expected="$1" desc="$2" msg="$3"
  local got
  printf '%s' "$msg" | "${VALIDATOR}" >/dev/null 2>&1
  got=$?
  if [[ "$got" -eq "$expected" ]]; then
    pass=$((pass + 1))
  else
    fail=$((fail + 1))
    printf 'FAIL: %s\n      expected exit %s, got %s\n' "$desc" "$expected" "$got"
  fi
}

# --- Valid conventional messages (exit 0) ---
assert 0 "plain type"                 "feat: add thing"
assert 0 "type with scope"            "fix(oci): wrap default transport"
assert 0 "breaking with bang"         "feat!: drop deprecated API"
assert 0 "breaking with scope + bang" "refactor(http)!: change signature"
assert 0 "chore type"                 "chore: update deps"
assert 0 "docs type"                  "docs: add godoc comments"
assert 0 "ci type"                    "ci: pin action to sha"
assert 0 "header + body"              $'feat: add retry\n\nAdds exponential backoff to the client.'

# --- Exempt messages that must be skipped (exit 0) ---
assert 0 "merge commit"      "Merge pull request #339 from robnester-rh/EC-2053"
assert 0 "git revert commit" 'Revert "feat: add retry"'
assert 0 "fixup autosquash"  "fixup! feat: add retry"
assert 0 "squash autosquash" "squash! feat: add retry"

# --- Invalid messages (non-zero) ---
assert 1 "no type prefix"        "Add resource limits to archive expanders"
assert 1 "unknown type"          "feature: add thing"
assert 1 "arbitrary word type"   "foo: bar"
assert 1 "missing colon"         "feat add thing"
assert 1 "missing space"         "feat:add thing"
assert 1 "empty subject"         "feat: "
assert 1 "empty message"         ""

printf '\n%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]]
