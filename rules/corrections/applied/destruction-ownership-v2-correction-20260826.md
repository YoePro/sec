# Correction: availability-aware partial and conditional destruction

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/memory/destruction.txt`

---

## Correction

Synchronize destruction with the revision-2 ownership availability model.

### Destruction follows current ownership

A former owner must never destroy a place after ownership has been moved,
discarded, detached, or otherwise ended.

For a partially available aggregate, cleanup destroys exactly the owned
sub-places that remain `Available` on the current path. Unavailable sub-places
are skipped. This rule is recursive for nested aggregates.

### Custom `free` and partial moves

In Sec 0.1, a type that defines custom `free` does not permit partial moves from
its fields. Custom whole-value destruction may depend on invariants across the
complete value; the compiler must not silently replace it with field-wise
cleanup after dismantling the value.

### Replacement

Assignment to an `Available` mutable place replaces its value and ends the old
value before installing the new one.

Assignment to an `Unavailable` mutable place is reinitialization and has no old
value to destroy.

For a hosted build, assignment to a `ConditionallyAvailable` mutable place is
legal: on paths that still own the old value it is ended before replacement; on
already-unavailable paths no old destruction occurs. The outgoing place is
`Available` after successful assignment.

A target/project policy that forbids dynamic ownership bookkeeping may require
explicit convergence such as:

```sec
discard package.Payload
package.Payload = CreateBuffer()
```

before such a replacement.

### Trivial destruction

Copy/move classification and destruction classification remain separate.
Whether cleanup code is required is determined by the resolved type's
trivial/non-trivial destruction classification and recursively by its owned
sub-values, not merely by whether it is copyable.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 owns availability and ownership
convergence; this rulebook owns concrete cleanup ordering/lowering.
