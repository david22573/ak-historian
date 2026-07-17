#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

./scripts/verify_source_integrity.sh
./scripts/test_source_integrity.sh
GOWORK=off go vet ./...
GOWORK=off go test ./...
GOWORK=off go build -buildvcs=false ./...
