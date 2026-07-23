# Exchange Metadata Snapshots

Exchange Metadata Snapshots record the state of exchange pairs at a given exact point in time. 

## Role in PIT Coverage
Snapshots form the foundation of historical survivorship bias removal. By proving that a symbol existed (or was removed) dynamically over a research window using collected point-in-time snapshots, ak-engine can guarantee researchers could have traded the symbol dynamically without peeking into the future.

Snapshots lacking an `observed_time` or captured with weak trust prevent strict RIF promotion.
