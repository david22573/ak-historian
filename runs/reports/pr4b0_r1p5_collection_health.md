# R1P5 collection health — source remediation required

Both receipt chains and the live collector are healthy, but the frozen source commit fails standalone reproduction. The readiness timer is disabled fail-closed so its non-reproducible binary cannot overwrite the remediation authority. See `pr4b0_r1p5_source_integrity_failure.json`.
