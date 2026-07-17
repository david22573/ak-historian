#!/usr/bin/env bash
set -euo pipefail

ROOT=${1:-$(cd "$(dirname "$0")/.." && pwd)}
ROOT=$(cd "$ROOT" && pwd)
REVISION=${SOURCE_INTEGRITY_REV:-HEAD}
cd "$ROOT"

test -d .git || git rev-parse --git-dir >/dev/null

all_go=$(mktemp)
tracked_go=$(mktemp)
archive_list=$(mktemp)
archive_root=$(mktemp -d)
trap 'rm -f "$all_go" "$tracked_go" "$archive_list"; rm -rf "$archive_root"' EXIT

find internal cmd pkg -type f -name '*.go' -print 2>/dev/null | LC_ALL=C sort >"$all_go"
git ls-files 'internal/**/*.go' 'cmd/**/*.go' 'pkg/**/*.go' | LC_ALL=C sort >"$tracked_go"

if ! diff -u "$tracked_go" "$all_go"; then
  echo "required Go sources differ from the tracked Git set" >&2
  exit 1
fi

while IFS= read -r source; do
  test -f "$source"
  git ls-files --error-unmatch -- "$source" >/dev/null
  if git check-ignore --no-index -q -- "$source"; then
    git check-ignore --no-index -v -- "$source" >&2 || true
    echo "required Go source is covered by an ignore rule: $source" >&2
    exit 1
  fi
done <"$all_go"

git ls-files --error-unmatch -- internal/r1p5/coverage.go >/dev/null
if git check-ignore --no-index -q -- internal/r1p5/coverage.go; then
  echo "internal/r1p5/coverage.go remains ignored" >&2
  exit 1
fi

git archive --format=tar "$REVISION" | tar -tf - | LC_ALL=C sort >"$archive_list"
while IFS= read -r source; do
  if ! grep -Fxq "$source" "$archive_list"; then
    echo "required Go source absent from Git archive: $source" >&2
    exit 1
  fi
done <"$all_go"

git archive --format=tar "$REVISION" | tar -xf - -C "$archive_root"
(
  cd "$archive_root"
  GOWORK=off go list -deps ./... >/dev/null
)

echo PR4B0_SOURCE_INTEGRITY_VALID
