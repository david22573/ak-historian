# R1P5 final decision — remediation required

Final label: `PR4B0_R1P5_COLLECTION_REMEDIATION_REQUIRED`.

The structural checkpoint is complete, but the receipt-bound source commit is not standalone-buildable because `internal/r1p5/coverage.go` was excluded by the repository's `coverage.*` ignore rule. No candidate implementation, result, partition, holdout, or RIF state was read or created.
