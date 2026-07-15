# PR4B0-R1P5 backfill source identity

Identity: `ak-historian.pr4b0-r1p5.backfill-source-identity.v1`

Hash: `sha256:90db4c16c95a8fcc5ed7fa331c59af92ba2740d91650af33e5c11232912531d1`

The immutable backfill source commit is `87354374fec9f52377e588575de35b072dd39924`. It contains the frozen R1P5 protocol, implementation, CLI, policies, supervisor templates, and tests and contains no real historical receipt. This exact commit and protocol hash must be embedded in every R1P5 receipt.

An earlier source identity was abandoned before any historical request after a performance audit found quadratic verification work. Its identity and disposition remain committed separately. No receipt binds the abandoned source.
