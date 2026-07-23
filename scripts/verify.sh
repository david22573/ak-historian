#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

bad_files="$(gofmt -l $(find . -path ./.git -prune -o -path ./.cache -prune -o -path ./runs -prune -o -path ./.ak-historian -prune -o -name '*.go' -print))"
if [ -n "$bad_files" ]; then
  printf 'gofmt required:\n%s\n' "$bad_files" >&2
  exit 1
fi

GOWORK=off go vet ./...
GOWORK=off go test ./...
GOWORK=off go build ./...
