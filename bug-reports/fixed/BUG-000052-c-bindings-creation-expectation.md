# BUG-000052 — Atomic C-binding creation did not recheck absence

## Status

Fixed on 2026-11-25.

## Report and reproduction evidence

Static audit of the previous CLI replacement helper found that `expected == nil`
was documented as requiring absence, but bypassed the final input check entirely.
If a file appeared after init's initial read and before replacement, the helper
could overwrite it. This pre-fix evidence is static; no timed race reproduction
is claimed.

## Expected and actual behavior

An absence expectation must reject an existing destination. The shared
replacement helper now checks absence immediately before rename, just as an
existing-file expectation rechecks its input SHA-256. Rejection preserves bytes
and removes the prepared temporary. Cooperating in-process writers serialize
the last check and replacement. Portable atomic rename does not supply a
cross-process compare-and-swap guarantee against arbitrary external writers.

## Scope

- Area: C-binding configuration file replacement.
- Rulebooks: MD-021 complete correction §5(2)–(4), `rules/tooling/cli.md` §8.
- Governance integration: `tooling.workspace-harness`.
- Affected phase: tooling filesystem mutation.

## Resolution

Extract the existing replacement implementation into the shared C-binding
service and enforce both present/hash and absent preconditions. Existing CLI
mutations and reviewed change plans use this implementation.

`TestChangePlanAtomicFailureCleansTemporary` verifies unchanged bytes and no
temporary leak after stale and absent-expectation failures.
`TestChangePlanValidationAndConcurrentCommit` verifies exactly one changed
commit from two in-process applications. Package race tests pass. These tests
cover the replacement protocol, not all host filesystem failure modes.
