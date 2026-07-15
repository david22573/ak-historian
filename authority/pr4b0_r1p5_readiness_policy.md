# PR4B0-R1P5 readiness policy

Identity: `ak-historian.pr4b0-r1p5.readiness-policy.v1`

Hash: `sha256:1dc7f4ef075304074da779a57e0650a497dd31cdc324813154acc5ac276e3c08`

Structural readiness requires at least 180 consecutive complete UTC days classified `UNEXPOSED_PIT_EVIDENCE_COMPLETE` for all nine symbols. Exact minute sequence, source schema, provider-time, synchronized-clock, receipt, raw, fragment, conflict, exposure, checkpoint, live-collector, watcher, and clean-verification gates all fail closed.

This floor does not waive any future research qualification gate. This phase reports only structural feasibility and never creates or names development, validation, or final-holdout partitions.
