# Sec MLIR Lowering — Planned Version v22

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-19
- **Last updated:** 2026-10-19
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/lowering-versions/sec_mlir_lowering_v22.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P26`

---
## § 1. Pipeline boundaries

Begin only after Sema/SemIR correctness, P15/P16 reference facts, P24 complete selected layout, P18/P19 storage transitions and P25 ABI plan where crossing calls. Lower physical references/descriptors to **standard MLIR**, not directly to invented LLVM pointer layout.

## § 2. Representation-policy resolution

`--sec-resolve-reference-representation` maps semantic reference requirements to one P24 selected physical representation and generated check plan. It is read-only with respect to Sema and layout; wrong/incomplete plan fails closed.

## § 3. Reference validity lowering

`--sec-lower-reference-validity` materializes required epoch/spatial/type checks and proven address-only references. Guarded success dominates access; invalid-reference-generation routes to canonical panic/trap. A guard cannot be hoisted beyond invalidating events.

## § 4. Descriptor lowering

`--sec-lower-selected-descriptors` materializes P24-selected slice, owned-array, string and Arena descriptor projections only where the corresponding high-level source semantics and storage plans are executable. Preserve owner/view distinction, current backing incarnation, address space and zero-sized element identities.

## § 5. P24 repr ops

Lower/refine `sec.repr.materialize`, `sec.repr.project_field`, `.project_element`, `.load_validated`, `.store_validated`, `.check_validity` and `.discharge` without duplicating Sema; do not emit a load before its required check or derive an invalid pointer before spatial validation.

## § 6. Epoch width and synchronization

Preserve the default 64-bit logical epoch on 32-bit systems. Use the selected atomic/critical-section/version protocol where concurrent invalidation is permitted. No torn multiword reads, silent wrap, missing target capability or global mandatory epoch manager.

## § 7. ABI and foreign calls

Native safe reference transport consumes P25 exact classification and complete representation. P25-approved call-bounded foreign ref params may adapt to raw address only with resolved proof; do not reconstruct safe refs from foreign returns or silently erase hidden generation components.

## § 8. Optimization and proof retention

A static proof can eliminate a check or metadata only while its exact guarantee remains valid. Preserve records for verifier/debug until the proof is discharged. Do not infer `nonnull`, `noalias`, `dereferenceable`, `lifetime` or alignment stronger than Sema/plan prove.

## § 9. Pass order

Suggested order:

```text
verify-Semantic-IR
verify-P15/P16/P18/P19
resolve-P24-layout
verify-P24-layout-obligations
resolve-P26-reference-representation
verify-P26-reference-physical-plans
lower-P26-reference-validity
lower-P26-selected-descriptors
verify-P26-reference-discharges
resolve/lower-P25-call-ABI when executable
standard-MLIR-verification
```

Actual operations/ordering must honor already installed passes, not pretend v21 ABI lowering is available.

## § 10. Completion boundary

Complete when all selected, actually supported reference/descriptor policies have verified physical lowering to standard MLIR, the stale reference failure is correct, no safety proof is silently erased, owner/ABI/FFI contracts survive and negative/cross-profile/cold-cache tests pass. LLVM dialect and final call ABI encoding remain deferred.
