# PR4B0-R1P4 supervisor contract

The durable mechanism is a rootless user-level systemd oneshot service launched by a persistent five-minute timer. The collector additionally holds a nonblocking file lock, rebuilds cursors from the verified receipt ledger, writes to the user journal, and relies on the next timer event for bounded recovery.

Committed source contains no machine path or secret. The installer creates local uncommitted unit files with the actual checkout, binary, activation, and data paths.
