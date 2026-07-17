# PR4B0-R1P5R preacquisition source seal

Exact source-seal commit `a81d343b3ae20c45c6d78f45f53cec67f8777442` passed isolated fresh-clone tidy, module-diff, vet, tests, race tests, build, `make verify`, archive/package-graph integrity, diff, credential, trader, order-endpoint, candidate-surface, absolute-path, sibling-dependency, and network-scope gates.

Verification ran from `2026-07-17T06:39:23Z` through `2026-07-17T06:46:35Z`, before any R1P5R receipt existed. The earlier no-request seal is preserved in Git history but superseded because its declared HTTP 408 retry behavior did not match the sealed client.

Two builds, including one with a fresh Go build cache, produced identical binary SHA-256 `c10fdf10255a8c88c817d5189b20ca7411be1fcd2ae64df8c07e2d1934054ae6`.
