# Transferability

## Purpose

Transferability defines which values and capabilities may cross task, thread or
process boundaries.

This rulebook is planned and does not define final syntax.

It must cover:

- move-only value transfer
- borrowed argument restrictions
- channel capability movement
- cross-thread-safe representations
- process IPC adapters
- target-specific restrictions

No value becomes transferable merely because it is used with `spawn thread`.
