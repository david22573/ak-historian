# PR4B0-R1P5 backfill source identity

Identity: `ak-historian.pr4b0-r1p5.backfill-source-identity.v1`

Hash: `sha256:002e87da844076dae1424f2d116fa364ac74051559853f38f1e2457d5386a228`

The immutable backfill source commit is `223f10c8653c4430dd283db020206b0634a101c5`. It contains the frozen R1P5 protocol, implementation, CLI, policies, supervisor templates, and tests and contains no real historical receipt. This exact commit and protocol hash must be embedded in every R1P5 receipt.

An earlier source identity was abandoned before any historical request after a performance audit found quadratic verification work. Its identity and disposition remain committed separately. No receipt binds the abandoned source.
