# PR4B0-R1P5 coverage protocol

Identity: `ak-historian.pr4b0-r1p5.coverage-protocol.v1`

Hash: `sha256:29c7c8d3ed89807cfb4affa768075fce39dc6be190a766e42691fba20384c91a`

This frozen protocol authorizes only public, unauthenticated Binance USD-M Futures one-minute historical reacquisition for the nine declared symbols. The eligible start is `2026-01-01T00:00:00Z`. Exact per-symbol ends join the first accepted P4 live receipt without an ambiguous gap. Backfill and live collection use separate binaries, roots, locks, cursors, and receipt chains.

Historical event and close timestamps are never treated as availability. Each receipt binds present-day response completion, provider HTTP Date, provider server time, synchronized local-clock evidence, exact raw bytes, the normalized fragment, the source commit, and this protocol. Raw and normalized bytes may be retained using deterministic lossless compression; verification hashes their exact uncompressed content.

The protocol permits structural coverage, acquisition evidence, exposure classification, immutable checkpoints, and readiness monitoring only. Candidate execution, candidate counts or outcomes, research partitions, holdout access, and real RIF state are prohibited.
