#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
DATA="$ROOT/runs/evidence/pr4b0_r1p4_activation"
ACTIVATION="$DATA/dataset_activation.json"
BINARY=${1:-"$ROOT/bin/ak-historian"}

test -x "$BINARY"
test -f "$ACTIVATION"

find "$ROOT/authority" "$ROOT/runs/reports" "$DATA" -type f -name '*.json' -print0 |
  xargs -0 -n1 jq -e . >/dev/null

"$BINARY" prospective-verify \
  --repository-root "$ROOT" \
  --data-root "$DATA" \
  --activation "$ACTIVATION" >/dev/null

test "$(find "$DATA/raw" -type f -name '*.json' | wc -l)" -eq 27
test "$(find "$DATA/normalized" -type f -name '*.json' | wc -l)" -eq 27
test "$(find "$DATA/receipts" -type f -name '*.json' | wc -l)" -eq 27
test "$(find "$DATA/manifests/partitions" -type f -name '*.json' | wc -l)" -eq 9
test "$(find "$DATA/manifests/checkpoints" -type f -name '*.json' | wc -l)" -eq 1

jq -s -e '
  length == 3 and
  all(.[]; .full_universe_success == true) and
  all(.[]; ([.symbols[] | select(.success == true)] | length) == 9) and
  ([.[].symbols[].normalized_record_count] | add) == 134 and
  ([.[].symbols[].duplicate_count] | add) == 0 and
  ([.[].symbols[].conflict_count] | add) == 0 and
  ([.[].symbols[].schema_failure_count] | add) == 0 and
  all(.[]; .clock_evidence.synchronized == true)
' "$DATA/ledgers/cycles.jsonl" >/dev/null

jq -s -e '
  length == 27 and
  .[0].prior_receipt_chain_hash == "sha256:0000000000000000000000000000000000000000000000000000000000000000" and
  .[-1].current_receipt_chain_hash == "sha256:44409b217367a292f7b9ab9d6bf48859ac0240199df31e2630a04589bfd2c6e6" and
  .[-1].accepted_authority_receipt.receipt_hash == "sha256:ab20b5ce22d52caa17e1f024e38b95736d1effdc8953acd9febbde20cf233aff" and
  all(.[]; .collector_source_commit == "598a9119be828daa7db76dacec017456807ccfed") and
  all(.[]; .protocol_hash == "sha256:671a27239d72e163428378dff926acc9f7a22036aff247cc8888ee9f06077311") and
  all(.[]; .local_clock_synchronization_evidence.synchronized == true) and
  all(.[]; .provider_http_date != "") and
  all(.[]; .provider_server_time_response_hash != "") and
  all(.[]; .availability_status == "PIT_ELIGIBLE")
' "$DATA/ledgers/receipts.jsonl" >/dev/null

for receipt in "$DATA"/receipts/date=*/*/*.json; do
  hash=$(jq -r .current_receipt_chain_hash "$receipt")
  jq -s -e --arg hash "$hash" 'any(.[]; .current_receipt_chain_hash == $hash)' "$DATA/ledgers/receipts.jsonl" >/dev/null
done

manifest_hashes=$(mktemp)
trap 'rm -f "$manifest_hashes"' EXIT
for manifest in "$DATA"/manifests/partitions/symbol=*/*.json; do
  recorded=$(jq -r .partition_hash "$manifest")
  actual="sha256:$(jq -cS 'del(.partition_hash)' "$manifest" | tr -d '\n' | sha256sum | awk '{print $1}')"
  test "$recorded" = "$actual"
  printf '%s\n' "$recorded" >>"$manifest_hashes"
  if jq -e '.. | strings | select(startswith("/"))' "$manifest" >/dev/null; then
    echo "absolute path in partition manifest: $manifest" >&2
    exit 1
  fi
done

checkpoint=$(find "$DATA/manifests/checkpoints" -type f -name '*.json')
recorded_checkpoint=$(jq -r .checkpoint_hash "$checkpoint")
actual_checkpoint="sha256:$(jq -cS 'del(.checkpoint_hash)' "$checkpoint" | tr -d '\n' | sha256sum | awk '{print $1}')"
test "$recorded_checkpoint" = "$actual_checkpoint"
diff -u <(sort "$manifest_hashes") <(jq -r '.partition_hashes[]' "$checkpoint" | sort)

for report in \
  "$ROOT/runs/reports/pr4b0_r1p4_activation_proof.json" \
  "$ROOT/runs/reports/pr4b0_r1p4_collection_health.json"; do
  recorded=$(jq -r .artifact_hash "$report")
  actual="sha256:$(jq -cS 'del(.artifact_hash)' "$report" | sha256sum | awk '{print $1}')"
  test "$recorded" = "$actual"
done

jq -e '
  .collector_source_commit == "598a9119be828daa7db76dacec017456807ccfed" and
  .activation_hash == "sha256:37bbb11677d07496b43fee24b4a84f12730713ef89506662015a53c04e8ef187" and
  .protocol_hash == "sha256:671a27239d72e163428378dff926acc9f7a22036aff247cc8888ee9f06077311"
' "$ACTIVATION" >/dev/null

echo PR4B0_R1P4_COMMITTED_EVIDENCE_VALID
