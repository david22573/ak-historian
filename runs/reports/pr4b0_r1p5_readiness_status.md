# R1P5 structural readiness — remediation required

The structural coverage gate passed with 195 complete days, but standalone source integrity failed because the receipt-bound source commit omits ignored `internal/r1p5/coverage.go`. The readiness label is therefore `PR4B0_R1P5_COLLECTION_REMEDIATION_REQUIRED`; the watcher is disabled fail-closed so it cannot overwrite this authority with a false ready result.
