# Sec MLIR Dialect — Planned Schema v22

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-19
- **Last updated:** 2026-10-19
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/dialect-versions/sec_mlir_dialect_v22.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P26`

---
## § 1. Version and predecessor condition

P26 targets **planned** `sec.dialect_version = 22 : i32`, successor to planned P25 schema v21. In the checked public rulebook the executable dialect is described as v11; v22 must not be claimed installed before actual prerequisite passes and verifiers exist.

## § 2. Reuse existing types

Reuse P15 `!sec.place`, `!sec.ref`/`!sec.ref_mut`, P16 slices, P18 owning array, P19 Arena and P24 selected physical representation types. Do not introduce a mandatory `!sec.fat_ref` or universal descriptor type.

## § 3. Typed plan and proof attributes

Add typed references equivalent to `sec.reference_physical_plan`, `sec.reference_proof_set`, `sec.reference_validation_plan`, `sec.descriptor_materialization_plan` and actual canonical LayoutID/RepresentationPolicyID. Reject mismatched compilation plans and missing provenance.

## § 4. Check-epoch operation

Proposed `sec.ref_repr.check_epoch` consumes a selected reference or planned domain/expected epoch, produces a Boolean/guard token, and refers to a typed validation plan. A checked failure lowers to `InvalidReferenceGeneration` panic/trap. Never turn stale direct-ref use into Result.

## § 5. Check-spatial operation

Proposed `sec.ref_repr.check_spatial` validates a selected index/range/extent before projection; it does not replace P14–P16 bounds semantics. It records original signedness and error kind. Reuse canonical bounds operations if actual implemented dialect already has an equivalent.

## § 6. Guarded access

Proposed `sec.ref_repr.guarded_access` (or canonical P24 `sec.repr.load_validated`/`store_validated`) requires explicit dominating proof/guard for all obligations. It does not create a new borrowing rule or imply a raw pointer address is safe.

## § 7. Selected descriptor materialization

Proposed `sec.descriptor.materialize_selected` and `.project_selected` consume the immutable layout/descriptor policy. They neither allocate by default nor fabricate owners. Reuse a semantically equivalent P24 op instead of introducing redundant TableGen operations.

## § 8. Compatibility and negative tests

Preserve schema v1–v21 regression parsing as supported by installed predecessors. Reject wrong policy, stale proof, wrong domain epoch, missing guard, check after deref, forged attributes, raw-to-safe reinterpret, invalid empty slice projection, mismatched ABI transport and unsupported profile.

## § 9. New verifier registrations

Register `--sec-verify-physical-references`, `--sec-verify-physical-descriptors` and `--sec-verify-reference-discharges`. Extend existing P15–P25 verifiers rather than creating a parallel safety authority.
