# PR4B0-R1P5 source integrity failure

The standalone fresh-clone gate failed after acquisition because `.gitignore` rule `coverage.*` excluded `internal/r1p5/coverage.go` from the receipt-bound source commit `59951efd756a8024455608d298c5534c778e5121`.

The local binary included the file and produced structurally complete evidence, but the committed source tree cannot reproduce that binary or build the R1P5 CLI. Because historical acquisition had begun, the frozen protocol forbids replacing, amending, or rebasing the bound source commit. The only honest final label is `PR4B0_R1P5_COLLECTION_REMEDIATION_REQUIRED`.
