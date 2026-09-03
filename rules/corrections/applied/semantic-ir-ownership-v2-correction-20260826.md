# Correction: canonical ownership and availability facts in Semantic IR

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/compiler/semantic_ir.md`

---

## Correction

Semantic IR must carry enough canonical ownership information to preserve
`rules/memory/ownership.md` revision-2 semantics without reconstructing them
from syntax after Sema.

At minimum, the semantic boundary must represent or make recoverable:

- canonical place identity and projection path;
- ownership transfer operations and their source place;
- explicit move-marker provenance where source semantics depend on it;
- canonical `Availability` state:
  `Uninitialized`, `Available`, `PartiallyAvailable`, `Unavailable`, and
  `ConditionallyAvailable`;
- separate unavailability reason/provenance such as moved, discarded, or
  detached;
- path-sensitive availability refinements from `is available` and
  `is not available`;
- partial aggregate ownership and still-owned sub-places;
- destruction/cleanup responsibility at exits and replacement points;
- ownership convergence produced by `discard`;
- delayed call-transfer commit for outer calls;
- explicit closure move capture;
- the distinction between static availability facts and runtime bookkeeping
  required only when static proof is insufficient.

Semantic IR must not encode `ConditionallyAvailable` as `Option`, nullability,
or generational-reference validity. These are distinct language concepts.

Backend lowering may choose SSA facts, local drop flags, control-flow splitting,
or another target-appropriate representation. That representation must not alter
public type/ABI layout unless another rulebook explicitly requires it.

A static-ownership target policy must be able to reject a program before
lowering if preserving source semantics would require forbidden dynamic
ownership bookkeeping.

## Cross-reference

`rules/memory/ownership.md` revision 2.0 is authoritative for ownership meaning;
Semantic IR preserves those facts for later lowering and analysis.
