#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

for file in \
  authority/pr4b0_r1p5_coverage_protocol.json \
  authority/pr4b0_r1p5_exposure_eligibility_policy.json \
  authority/pr4b0_r1p5_readiness_policy.json; do
  jq -e . "$file" >/dev/null
done

test "$(jq -r .protocol_hash authority/pr4b0_r1p5_coverage_protocol.json)" = \
  "sha256:$(jq -cS 'del(.protocol_hash)' authority/pr4b0_r1p5_coverage_protocol.json | sha256sum | awk '{print $1}')"
for file in authority/pr4b0_r1p5_exposure_eligibility_policy.json authority/pr4b0_r1p5_readiness_policy.json; do
  test "$(jq -r .policy_hash "$file")" = \
    "sha256:$(jq -cS 'del(.policy_hash)' "$file" | sha256sum | awk '{print $1}')"
done

GOWORK=off go vet ./...
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go build -buildvcs=false ./...

if rg -n --glob '!**/*_test.go' 'ak-trader|/fapi/v1/order|api[_-]?key|secret[_-]?key|testnet' internal/r1p5 internal/app/r1p5.go config/systemd/ak-historian-readiness-watch.*; then
  echo "forbidden credential, trader, or order surface" >&2
  exit 1
fi
if rg -n --glob '!**/*_test.go' 'profit.factor|expectancy|win.rate|drawdown|candidate.event|candidate.cluster|holdout.identity|development.partition|validation.partition' internal/r1p5 internal/app/r1p5.go authority/pr4b0_r1p5_*; then
  echo "candidate metric or research partition surface" >&2
  exit 1
fi

git diff --check
echo PR4B0_R1P5_SOURCE_VALID
