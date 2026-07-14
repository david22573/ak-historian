# PR4B0-R1P4 availability policy

`observed_available_at_utc` is the conservative maximum of the complete response time, parsed provider HTTP `Date`, and the cycle's provider server time.

Physical bytes without complete provider-time, synchronized-clock, raw-hash, schema, and durable receipt evidence are retained only as `AVAILABILITY_EVIDENCE_INCOMPLETE` and are never PIT eligible. No timestamp, filename, filesystem metadata, Git timestamp, report timestamp, or expected publication schedule can substitute for a real receipt.
