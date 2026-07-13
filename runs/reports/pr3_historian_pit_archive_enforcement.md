# PR3 — Historian PIT Archive Enforcement

## Executive verdict

Complete. `ak-historian` now produces strict point-in-time eligibility only after it parses a mandatory versioned snapshot manifest, verifies the real bounded snapshot bytes under an approved archive root, proves exact declared coverage, enforces source availability at an explicit historical cutoff, and validates a tamper-evident evidence envelope bound to those results.

Final label: `PR3_HISTORIAN_PIT_ARCHIVE_ENFORCEMENT_COMPLETE`

No candidate was promoted. RIF lifecycle policy, engine paper strategies, engine grading, trader execution, and mainnet controls were not changed.

## Baseline and result

- Accepted baseline: `241648e29b9e31a588b52ae8e0aeb58c6f7535b8`
- Required accepted ancestors: `73d2e7d09f821ec330508c663f4baded7fcb15e8`, `b91f7b4bc039091001ff9b5a2dc01414154db8eb`, `241648e29b9e31a588b52ae8e0aeb58c6f7535b8`
- Resulting implementation commit: `695aac277e4127b7efb247cac5ff5da7ca6287a1`
- Branch: `pr3-historian-pit-archive-enforcement`
- Generated: `2026-07-13T17:51:49Z`

The report artifact is committed after the implementation commit, so its own content-addressed commit cannot contain its final self hash. The exact report commit is available from `git log -1 -- runs/reports/pr3_historian_pit_archive_enforcement.md`.

## Manifest and snapshot schemas

Snapshot-manifest schema: `ak-historian.snapshot-manifest.v1`.

Mandatory manifest semantics:

- stable manifest, dataset, dataset-version, source, and archive identities;
- mandatory `[research_window_start,research_window_end)` and `generated_at` timestamps;
- versioned coverage and availability policies;
- positive snapshot count exactly matching the references;
- deterministic canonical `sha256:` manifest hash;
- unknown fields and schemas fail closed.

Snapshot-reference schema: `ak-historian.snapshot.v1`. Each reference binds a stable snapshot ID, partition key, clean archive-relative path, exact `[event_time_start,event_time_end)`, source `available_at`, separate optional `ingested_at`, SHA-256 content hash, schema version, and positive byte size.

## Coverage policy

Coverage-policy schema: `ak-historian.coverage-policy.v1`.

Expected coverage comes only from an explicit list of required partition keys and exact event-time intervals. The policy declares its partition model, nonnegative maximum permitted gap, and any closure or unavailable-source exceptions with stable IDs, bounds, and reasons. Required intervals plus exceptions must span the entire research window, including partial first and last periods, without overlaps or excessive gaps.

Structured output contains expected, observed, missing, duplicate, and out-of-window partitions; coverage ratio; maximum observed and permitted gaps; and strict verdict. Endpoint span cannot hide an internal gap. Undeclared closures fail; declared closures pass only when the remaining required partitions are complete.

## Availability policy

Availability-policy schema: `ak-historian.availability-policy.v1`.

The cutoff is mandatory and inclusive. Every snapshot must satisfy:

```text
available_at >= event_time_end + required_publication_delay
available_at <= evaluation_cutoff
```

Event and availability timestamps must not be future-dated. A missing source availability timestamp fails even when an ingestion timestamp exists. `generated_at` and `ingested_at` never substitute for `available_at`.

## Snapshot hash verification

The evaluator accepts only clean relative paths and rejects traversal, backslash ambiguity, temporary names, nonregular files, and symlink components. `os.OpenRoot` confines access to the approved archive root. Each opened file is size-checked, streamed through a bounded SHA-256 calculation, compared against both declared and opened-file sizes, and digest-compared in constant time. The evaluator does not trust a digest merely because it appears in the manifest.

## Evidence envelope and canonical hashing

Evidence schema: `ak-historian.pit-evidence.v1`.

The envelope binds evidence, dataset/version, window, cutoff, manifest/hash, archive, coverage policy and result, snapshot integrity counts, accepted snapshot-set digest, availability result, final verdict, strict-promotion flag, generation time, and historian build. Evidence verification recomputes the hash and checks internal approval/count/set invariants.

Canonical hashing uses compact JSON, UTC RFC3339Nano timestamps, stable sorting of snapshots/partitions/exceptions, and SHA-256 with an explicit `sha256:` algorithm prefix. Machine-specific absolute paths are excluded from evidence hashes. The snapshot-set digest covers accepted snapshot identities, partitions, temporal bounds, availability, schema, sizes, and content hashes. These artifacts are integrity-hashed, digest-verified, tamper-evident, and content-addressed; they are not asymmetrically signed.

## Verdict semantics

- `PIT_ELIGIBLE`: all strict manifest, snapshot, coverage, availability, and evidence-integrity checks passed.
- `PIT_INELIGIBLE`: structurally valid evidence violates coverage or temporal eligibility.
- `PIT_EVIDENCE_INCOMPLETE`: required evidence is absent.
- `PIT_EVIDENCE_CORRUPT`: schema, identity, path, size, digest, or integrity invariants fail.
- `PIT_DIAGNOSTIC_ONLY`: explicit non-strict result; never strict promotion evidence.

`strict_promotion_allowed` is true exactly for `PIT_ELIGIBLE` with all bound checks passing and a valid evidence integrity hash.

## Atomic writes

Manifest, evidence, and evaluation-result writes create a same-directory temporary file, write bounded complete content, flush, close, apply `0644` permissions, atomically rename, and sync the directory. Pre-rename failures preserve the prior valid artifact. Temporary manifest and snapshot names are rejected, and authoritative artifacts are not world-writable.

## Structured failure codes

Manifest: `MANIFEST_MISSING`, `MANIFEST_EMPTY`, `MANIFEST_UNREADABLE`, `MANIFEST_MALFORMED`, `MANIFEST_SCHEMA_UNSUPPORTED`, `MANIFEST_FIELD_MISSING`, `MANIFEST_DATASET_MISMATCH`, `MANIFEST_WINDOW_MISMATCH`, `MANIFEST_HASH_MISMATCH`, `MANIFEST_SNAPSHOT_COUNT_MISMATCH`.

Snapshot: `SNAPSHOT_MISSING`, `SNAPSHOT_HASH_MISMATCH`, `SNAPSHOT_DUPLICATE_ID`, `SNAPSHOT_PARTITION_CONFLICT`, `SNAPSHOT_SCHEMA_UNSUPPORTED`, `SNAPSHOT_PATH_INVALID`, `SNAPSHOT_TOO_LARGE`, `SNAPSHOT_SIZE_MISMATCH`.

Coverage/availability: `COVERAGE_POLICY_UNSUPPORTED`, `COVERAGE_POLICY_INVALID`, `COVERAGE_INCOMPLETE`, `AVAILABILITY_TIMESTAMP_MISSING`, `AVAILABLE_AFTER_EVALUATION`, `PUBLICATION_DELAY_VIOLATION`, `FUTURE_SNAPSHOT_TIMESTAMP`, `EVALUATION_CUTOFF_MISSING`.

Evidence: `EVIDENCE_SCHEMA_UNSUPPORTED`, `EVIDENCE_INTEGRITY_HASH_MISMATCH`, `EVIDENCE_STRICT_PROMOTION_INVALID`, `EVIDENCE_SECURITY_FIELD_MISSING`.

## Former defect regression

The accepted PR1 baseline had no PIT evaluator; existing coverage and dataset manifests could not establish strict archive-backed PIT evidence. The new regression proves that a perfect declared window with no manifest returns `PIT_EVIDENCE_INCOMPLETE`, never `PIT_ELIGIBLE`. The app-level command test runs the same request with a real manifest/snapshot fixture and then a missing `--snapshot-manifest` target, proving the argument is observably consumed. Every test that obtains `PIT_ELIGIBLE` uses real manifest and snapshot bytes.

## Tests added

- Manifest: valid, missing, empty, malformed, unknown field/schema, malformed timestamp, missing identity, window mismatch, hash mismatch, zero/count mismatch, duplicate IDs, oversized input, hash-algorithm confusion, and temporary-file rejection.
- Snapshot: valid hashes, missing, modified, truncated, conflicting partition, traversal, symlink escape, unsupported schema, and oversized read.
- Coverage: complete, missing first/middle/final, duplicate, out-of-window, endpoint-span internal gap, declared/undeclared closure, maximum observed gap, and partial noneligibility.
- Availability: before/exactly-at/after cutoff, missing availability, ingestion non-substitution, future timestamp, and publication-delay violation.
- Evidence: valid verification plus dataset, window, manifest hash, snapshot set, coverage, availability, final verdict, schema, and strict-approval mutations.
- Persistence/compatibility: atomic permissions, failed-write preservation, temporary cleanup, versioned future-RIF evidence fixture, and end-to-end CLI manifest consumption.

Existing non-PIT historian tests remain green. No RIF lifecycle policy or engine/trader dependency was introduced.

## Security review

| Finding | Resolution | Status |
|---|---|---|
| Path traversal/archive escape | Clean relative-path validation plus rooted `os.OpenRoot` operations | Resolved |
| Symlink escape | Reject every symlink component; rooted open remains the confinement boundary | Resolved |
| Manifest substitution | Bind manifest ID/hash, expected dataset/version/window, and archive ID | Resolved |
| Hash confusion | Accept only exact `sha256:` digests with 32 decoded bytes; constant-time comparison | Resolved |
| Duplicate identities/conflicting partitions | Stable ID checks and exact one-to-one required-partition comparison | Resolved |
| Oversized/unbounded input | 4 MiB manifest/evidence limits and configurable per-snapshot bounded streaming | Resolved |
| Malformed timestamps/counts/integer overflow | Strict JSON decoding, mandatory nonzero times, ordered bounds, exact counts, duration overflow limits | Resolved |
| TOCTOU | Hash and size checks use the opened rooted file descriptor; `os.OpenRoot` protects root resolution on the verified Linux host | Resolved for verified host |
| Temporary/partial authority | Atomic same-filesystem writes and temporary-name rejection | Resolved |
| Log/path leakage | Failure messages contain stable IDs/reasons, not archive absolute paths; path scan is clean | Resolved |
| Fail-open defaults | Strict mode is default; missing/unknown/corrupt evidence cannot produce eligibility; diagnostics use a distinct verdict | Resolved |
| Generated/ingestion time trusted as availability | Separate mandatory source `available_at` validation | Resolved |
| False signature terminology | Documentation explicitly uses integrity-hashed/tamper-evident terminology | Resolved |

## Verification commands and exit codes

Accepted linked worktree:

| Command | Exit | Result |
|---|---:|---|
| `gofmt -w <changed-go-files>` | 0 | Pass |
| `GOWORK=off go mod tidy` | 0 | Pass |
| `git diff --exit-code -- go.mod go.sum` | 0 | Pass |
| `GOWORK=off go vet ./...` | 0 | Pass |
| `GOWORK=off go test ./...` | 0 | Pass |
| `GOWORK=off go test -race ./...` | 0 | Pass |
| `GOWORK=off go build ./...` | 1 | Linked-worktree Go VCS-root limitation |
| `GOWORK=off make verify` | 2 | Vet/test pass, then same linked-worktree VCS-root limitation |
| `git diff --check` | 0 | Pass |

The local build limitation is precisely reproduced by `go build -v`: Go 1.25 ignores the linked worktree's `.git` file, runs `git status --porcelain` from the workspace parent, and receives exit 128. It is not a compile or test failure.

Fresh standalone clone at `695aac277e4127b7efb247cac5ff5da7ca6287a1`, with no sibling AK repository:

| Command | Exit | Result |
|---|---:|---|
| `git clone --no-local --branch pr3-historian-pit-archive-enforcement <source> <isolated>` | 0 | Pass |
| `gofmt -w <changed-go-files>` | 0 | Pass |
| `GOWORK=off go mod tidy` | 0 | Pass |
| `git diff --exit-code -- go.mod go.sum` | 0 | Pass |
| `GOWORK=off go vet ./...` | 0 | Pass |
| `GOWORK=off go test ./...` | 0 | Pass |
| `GOWORK=off go test -race ./...` | 0 | Pass |
| `GOWORK=off go build ./...` | 0 | Pass |
| `GOWORK=off make verify` | 0 | Pass |
| `git diff --check` | 0 | Pass |
| `git status --short` | 0, empty | Pass |
| lightweight secret scan | 0 matches | Pass |
| absolute-path contamination scan | 0 matches | Pass |
| sibling `ak-*` repository scan | 0 matches | Pass |

## Files changed

`README.md`, `docs/pit_archive_evidence.md`, `internal/app/pit.go`, `internal/app/pit_test.go`, all new files under `internal/pitarchive/`, and `testdata/pitarchive/v1/evidence.example.json`. The implementation commit changes 19 files with no module dependency change.

## Compatibility and deferred work

`testdata/pitarchive/v1/evidence.example.json` is a versioned, integrity-valid, path-independent envelope suitable for future RIF consumption. RIF must later validate the version, identities, integrity hash, and strict-approval invariant before applying its own lifecycle policy.

Deferred intentionally: RIF lifecycle consumption, candidate promotion, paper-strategy implementation, engine grading and signal changes, trader integration/execution, mainnet controls, asymmetric signatures/key management, and repository-hygiene cleanup.

Recommended next PR: `PR4_ENGINE_STRATEGY_FAITHFUL_PAPER_EVALUATION`.
