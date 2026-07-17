#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

EXPECTED_COMMIT=${1:-$(git rev-parse HEAD)}
test "$(git rev-parse HEAD)" = "$EXPECTED_COMMIT"
test -z "$(git status --short)"

for file in \
  authority/pr4b0_r1p5r_reacquisition_protocol.json \
  authority/pr4b0_r1p5r_source_identity.json \
  authority/pr4b0_r1p5r_abandoned_evidence_registry.json; do
  jq -e . "$file" >/dev/null
done

./scripts/verify_source_integrity.sh
GOWORK=off go mod tidy
git diff --exit-code -- go.mod go.sum
GOWORK=off go vet ./...
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go build ./...
GOWORK=off make verify
git diff --check

if rg -n --glob '!**/*_test.go' -i 'ak-trader|/fapi/v1/order|api[_-]?key|secret[_-]?key|credential|password|testnet' internal/r1p5r internal/app/r1p5r.go config/systemd/ak-historian-r1p5r-readiness-watch.*; then
  echo "forbidden credential, trader, or order surface" >&2
  exit 1
fi
if rg -n --glob '!**/*_test.go' -i 'downtrendmidvolrelieflong240m|profit.factor|expectancy|win.rate|drawdown|candidate.event|candidate.cluster|holdout.identity|development.partition|validation.partition' internal/r1p5r internal/app/r1p5r.go authority/pr4b0_r1p5r_*; then
  echo "candidate metric or research partition surface" >&2
  exit 1
fi
if rg -n '"net/http"|http\.NewRequest|http\.Client' internal/r1p5r internal/app/r1p5r.go; then
  echo "network implementation escaped the approved prospective client" >&2
  exit 1
fi
if rg -n '/home/|/Users/|[A-Za-z]:\\\\' authority/pr4b0_r1p5r_* config/systemd/ak-historian-r1p5r-readiness-watch.*; then
  echo "absolute path entered source authority" >&2
  exit 1
fi

echo PR4B0_R1P5R_SOURCE_VALID
