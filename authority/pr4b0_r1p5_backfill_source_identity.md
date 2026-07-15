# PR4B0-R1P5 backfill source identity

Identity: `ak-historian.pr4b0-r1p5.backfill-source-identity.v1`

Hash: `sha256:531c1a5e0aeaa283422ecf5c652080aca6da99f30f58ac4e3e7c918f1948538b`

The immutable backfill source commit is `59951efd756a8024455608d298c5534c778e5121`. It contains the frozen R1P5 protocol, implementation, CLI, policies, supervisor templates, and tests and contains no real historical receipt. This exact commit and protocol hash must be embedded in every R1P5 receipt.

An earlier source identity was abandoned before any historical request after a performance audit found quadratic verification work. Its identity and disposition remain committed separately. No receipt binds the abandoned source.
