# Historian point-in-time archive evidence

`ak-historian` owns archive validation, snapshot-byte validation, coverage proof, temporal availability, and PIT verdict generation. It does not promote candidates or apply RIF lifecycle policy.

## Versioned schemas

The manifest schema is `ak-historian.snapshot-manifest.v1`. Required top-level fields are:

- `manifest_id`, `dataset_id`, `dataset_version`, `source`, and `archive_id`;
- RFC3339 `research_window_start`, `research_window_end`, and `generated_at`;
- a versioned `coverage_policy` and `availability_policy`;
- a positive `snapshot_count` equal to the number of `snapshots`;
- a canonical SHA-256 `manifest_hash`.

Every snapshot uses schema `ak-historian.snapshot.v1` and identifies `snapshot_id`, `partition_key`, a clean archive-relative path, `[event_time_start,event_time_end)`, source `available_at`, optional distinct `ingested_at`, `sha256:` content digest, and positive byte size. Unknown JSON fields, unknown schemas, zero values, duplicate identities, unsupported digests, and count mismatches fail closed.

The evidence-envelope schema is `ak-historian.pit-evidence.v1`. It binds the evidence and dataset identities, window and cutoff, manifest and archive identities, coverage-policy version and full coverage result, snapshot verification counts and accepted-set digest, availability result, final verdict, strict-promotion boolean, generation time, and historian build identity. The compatibility example at `testdata/pitarchive/v1/evidence.example.json` contains no absolute paths and can be consumed by a future RIF change.

## Coverage policy

Version `ak-historian.coverage-policy.v1` declares exact required partition keys and their `[start,end)` intervals. It also declares the partition model, a nonnegative maximum permitted gap, and any source closure or maintenance intervals with stable exception IDs and reasons.

Required partitions plus declared exceptions must span the complete research window without overlaps or a gap larger than the declared maximum. This makes partial first and last intervals explicit. Files present on disk never define expected coverage.

The evaluator reports sorted expected, observed, missing, duplicate, and out-of-window partition keys; a six-decimal coverage ratio; maximum observed and permitted gap seconds; and the strict verdict. A snapshot counts only when its partition key and exact event-time bounds match a declared required partition. Internal gaps fail even when first and last snapshots span the requested window. An exchange closure or unavailable-source period is accepted only when the manifest policy declares it.

## Availability policy

Version `ak-historian.availability-policy.v1` requires a policy ID and an explicit nonnegative publication delay. A snapshot passes only when:

```text
available_at >= event_time_end + required_publication_delay
available_at <= evaluation_cutoff
event and availability timestamps are not future-dated
```

The cutoff comparison is inclusive. Missing `available_at` fails even if `ingested_at` exists; generation and ingestion times never substitute for source availability.

## Snapshot verification

Snapshot paths must be clean relative slash-separated paths. Absolute paths, traversal, backslash ambiguity, temporary names, nonregular files, and symlink components are rejected. `os.OpenRoot` confines reads to the approved archive root, including concurrent symlink resolution on supported host platforms.

Each file is opened under that root, size-checked, streamed through a bounded SHA-256 read, checked against both the declared and opened-file sizes, and compared to the manifest digest in constant time. Missing, modified, truncated, oversized, duplicate, conflicting, and unsupported snapshots fail strict PIT. The integrity check hashes the bytes from the opened file descriptor, so a separate pre-hash trust decision is not used.

## Canonical hashing

Canonical hashes use compact JSON and SHA-256 with the `sha256:` prefix. Times are normalized to UTC RFC3339Nano. Snapshot references, required partitions, and exceptions are sorted by stable identity and time before hashing. The manifest hash excludes only `manifest_hash` itself.

The snapshot-set digest covers each accepted snapshot identity, partition, event bounds, availability, content hash, schema, and byte size. It intentionally excludes the machine-specific archive path and nonsecurity ingestion path metadata.

The evidence integrity hash excludes only `integrity_hash` itself and covers every evidence field, including the dataset, window, cutoff, manifest/archive identity, accepted snapshot-set digest, complete coverage and availability results, final verdict, strict-promotion flag, generation timestamp, and historian build. These are integrity hashes: they are tamper-evident and content-addressed, not asymmetric signatures.

## Verdicts

- `PIT_ELIGIBLE`: every strict manifest, byte-integrity, coverage, availability, and evidence-integrity check passed.
- `PIT_INELIGIBLE`: structurally valid evidence fails a temporal or coverage rule.
- `PIT_EVIDENCE_INCOMPLETE`: mandatory evidence is absent.
- `PIT_EVIDENCE_CORRUPT`: a schema, identity, path, size, digest, or integrity invariant fails.
- `PIT_DIAGNOSTIC_ONLY`: explicit non-strict analysis; never reusable as eligibility.

`strict_promotion_allowed` is true exactly when the final verdict is `PIT_ELIGIBLE` and all bound check verdicts pass. Consumers must validate this invariant and the integrity hash; they must not infer approval from free-form text.

## Atomic persistence

Authoritative manifests, evidence envelopes, and evaluation results are persisted by creating a same-directory temporary file, applying non-world-writable permissions, writing complete bounded content, syncing, closing, atomically renaming, and syncing the containing directory. Pre-rename errors preserve the prior artifact, and temporary names are never accepted as manifests or snapshots.
