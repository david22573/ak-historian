# PR4B0-R1P5R source integrity

The ignored `internal/r1p5/coverage.go` bytes were preserved before repair at SHA-256 `554e0514f2cb65ccd1c2da543de9983f95580605f9fdbd21d574285f73de128c`. Filesystem timestamps, mode, owner, and inode were recorded only as non-authoritative forensic metadata.

The exact file was reviewed against every tracked caller. It contains structural coverage, hashing, immutable checkpoint/report writes, and local systemd-state inspection; it contains no candidate execution, outcome calculation, credential, order, trader, hidden network, absolute-path, or random identity logic.

The failed binary embeds the old source commit, omitted source path, and compiled coverage symbols and predates the first receipt, which is strong best-available evidence that the file was compiled. Because the old commit cannot reproduce that binary, this is not overstated as cryptographic proof.

Repair implementation `fba40dc23a24afdc2ac03b76e5a2609df4b6116d` and replacement source seal `a81d343b3ae20c45c6d78f45f53cec67f8777442` pass the complete source-integrity and isolated fresh-clone gates. The first seal is retained in Git history and explicitly superseded before any request because its retry declaration differed from the client. No R1P5R receipt existed before replacement seal completion.
