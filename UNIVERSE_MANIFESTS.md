# Universe Manifests

Universe manifests dictate what assets are eligible for trading dynamically in time.

## PIT Integration
When building or updating a universe manifest, the `pit-evidence-coverage-report` can be supplied to embed the PIT eligibility hash directly into the artifact. This makes the universe explicitly tied to its survivorship-bias verification status. If the universe's symbols lack cryptographic evidence for their listing/delisting constraints, the universe is downgraded or marked ineligible for strict RIF testing downstream.
