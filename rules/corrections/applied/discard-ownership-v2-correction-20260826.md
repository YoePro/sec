# Correction: discard as ownership convergence

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/control-flow/discard.md`

---

## Correction

Synchronize `discard` place semantics with `rules/memory/ownership.md` revision
2.0.

### `discard place` converges to `Unavailable`

For a tracked place, `discard` means that the current owner intentionally ends
any remaining ownership in that place and that the outgoing ownership state is
`Unavailable`.

The behavior depends on incoming availability:

```text
Available
    destroy/end the owned value when required
    -> Unavailable / Discarded

Unavailable
    no-op
    -> Unavailable

ConditionallyAvailable
    destroy/end only on runtime paths that still own the value
    do nothing on paths where it is already unavailable
    -> Unavailable on every outgoing path
```

A second discard of an already unavailable place is therefore not an ownership
error. It is a legal no-op. Tooling may report a low-priority redundant-operation
advisory when this is statically obvious, subject to the existing diagnostic
policy.

### Restrictions remain

This correction does not permit discarding a value that carries a semantic
must-handle obligation that the discard rulebook forbids, nor does it bypass an
incompatible active borrow. Discardability and borrow legality are checked
before ownership convergence commits.

### Bare-metal/static-ownership use

`discard` is the canonical explicit convergence operation when code must turn a
conditionally available place into a definite unavailable state before
reinitialization or another operation requiring static ownership certainty.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 owns availability state and destruction
responsibility after discard.
