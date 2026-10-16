# P24 Semantic IR Amendment — Resolved Layout and Physical Projection

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_semantic_ir_layout_package24.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P24`

---

## § 1. Canonical layout service

The Semantic IR builder consumes read-only resolved facts. A typed `ResolvedLayoutRef` references one immutable record indexed by `(TypeID, CompilationPlanLayoutKey, LayoutContractID)`; it must not calculate physical offsets or inspect compiler-host type sizes.

## § 2. Complete/incomplete states

Target-independent Semantic IR may retain an unresolved `LayoutRequirementRef`. Executable layout-sensitive IR requires complete `ResolvedLayoutRef`, with source-plan identity attached. Failure is `unavailable/unsupported/invalid layout`, not guessed fallback.

## § 3. Layout contents

Required: semantic type and source provenance, plan identity, exact size/align, declaration-order fields and offsets, padding, array stride, union tag/payload, selected descriptor/reference policy, validity constraints, stability/contract provenance, address space.

## § 4. Layout identity

`LayoutID` must have deterministic snapshot identity; type names and numeric addresses are not IDs. Existing cached layout from incompatible plan cannot be used as truth.

## § 5. Semantic IR static queries

Support `LayoutRequire`, `LayoutResolve`, `LayoutSizeOf`, `LayoutAlignOf`, `LayoutStrideOf`, `LayoutFieldOffset` as analysis facts or static-only ops. Queries are target-plan bound and never require runtime allocations.

## § 6. Representation operations

Support `RepresentationMaterialize`, `RepresentationProjectField`, `RepresentationProjectElement`, `RepresentationProjectVariant`, `RepresentationLoadValidated`, `RepresentationStoreValidated`, `RepresentationCheckValidity`, `RepresentationDischarge` with exact layout/proof IDs. Keep a minimal set by reusing existing operation equivalents.

## § 7. Layout effects and ownership

Materialization itself may need storage, but cannot silently introduce dynamic allocation. It may move/copy a value only under the existing P17 resolved action. Pointer formation preserves P15 borrow and storage-domain facts.

## § 8. Valid and invalid bit patterns

Physical read into semantic T must carry a proof or runtime validation of bool/rune/enum/named/ref/active payload invariants. Uninitialized bytes and padding never become safe T.

## § 9. No physical ABI

Store-only physical representation does not choose function parameter/return ABI, foreign compatibility or raw serialization format.

## § 10. Verification and printer

Deterministic textual form exposes `LayoutID`, `CompilationPlan` fingerprint, precise offset/stride/size/align and outstanding obligations. Verify stable ordering and no dangling/wrong-plan references.
