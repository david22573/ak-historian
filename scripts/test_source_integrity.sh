#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TEMP_PARENT=$(mktemp -d)
CLONE="$TEMP_PARENT/historian"
trap 'rm -rf "$TEMP_PARENT"' EXIT

git clone --quiet --no-local "$ROOT" "$CLONE"
"$CLONE/scripts/verify_source_integrity.sh" "$CLONE" >/dev/null

rm "$CLONE/internal/r1p5/coverage.go"
if "$CLONE/scripts/verify_source_integrity.sh" "$CLONE" >/dev/null 2>&1; then
  echo "source-integrity verification accepted a deleted tracked source" >&2
  exit 1
fi

git -C "$CLONE" restore -- internal/r1p5/coverage.go
mkdir -p "$CLONE/internal/sourceintegrityignored"
printf 'package sourceintegrityignored\n' >"$CLONE/internal/sourceintegrityignored/required.go"
printf 'internal/sourceintegrityignored/required.go\n' >>"$CLONE/.git/info/exclude"
if "$CLONE/scripts/verify_source_integrity.sh" "$CLONE" >/dev/null 2>&1; then
  echo "source-integrity verification accepted an ignored required source" >&2
  exit 1
fi

echo PR4B0_SOURCE_INTEGRITY_NEGATIVE_TESTS_VALID
