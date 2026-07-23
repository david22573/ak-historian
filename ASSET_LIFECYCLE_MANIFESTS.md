# Asset Lifecycle Manifests

Asset Lifecycle manifests consolidate all sources of evidence into a single canonical view of a symbol's lifespan.

## Components
- **Evidence Level**: Dictates whether a symbol's lifecycle dates are trusted for strict promotion.
- **Listing/Delisting Status**: Confirms whether the edges of the lifecycle were verified.

## PIT Usage
The `pit-evidence-coverage` command reads the lifecycle manifest and matches it against the dataset/universe bounds. Unverified lifecycles produce warnings that pass down to the universe and dataset manifests, ultimately blocking strict RIF promotion.
