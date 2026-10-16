# P24 Normative Synchronization — Canonical Resolved Layout

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_layout_sync_package24.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P24`

---

## § 1. Authority and non-renumbering

`rules/memory/layout.md` is already normative and includes §§ 1–46. Do **not** replace or renumber these paragraphs. Append/synchronize only cross-document implementation requirements. The compiler maintains one `ResolvedLayout` service per concrete `CompilationPlan`; MLIR/LLVM consume it and cannot add their own layout rules.

## § 2. Plan-dependent scalar widths

`int`/`uint` use the selected plan width. Plain `float` uses binary32 on 32-bit and binary64 on 64-bit per MD-014 (`layout.md §18(4a)`). Fixed float32/float64 retain their widths. The old P20 float=binary64-always plan requires compatibility gating; P24 does not rewrite P20.

## § 3. Decimal layout

`layout.md §19` already fixes semantic composite representation `{ i64,i32 }` for `decimal` and `{ i128,i32 }` for `decimal128`. Target-selected alignment/field offsets remain resolved layout facts, not ABI classification.

## § 4. Layout is not storage/ABI

Layout describes representation, not ownership, lifetime, reclamation, allocation policy, FFI compatibility, or argument passing. A descriptor is sized independently of its backing. A safe ref may need metadata unlike `RawPtr[T]`.

## § 5. Shared plan keys

Resolved layouts and caches include all relevant `CompilationPlan` identity and generic substitutions. No host-size fallback or timestamp-derived validity. Wrong-plan reuse must fail closed.

## § 6. Representation validity

Padding is not semantic. A valid aligned bit pattern is not automatically a valid T. Defer validity, ownership and borrow obligations until checked/proven or materialized.

## § 7. Existing unresolved source syntax

The source syntax of explicit packing/offset/align constraints belongs to **existing MD-019**, not a new P24-D#. The service accepts already approved validated constraints but cannot invent parser spelling.

## § 8. Existing P23 dependencies

Public string offsets/signatures/char conversions remain P23-D1, P23-D2, P23-D3. Invalid UTF-8 remains MD-050. Do not resolve these by choosing a descriptor layout.

## § 9. Governance sync

Update `governance/memory.yaml`: `frontend.layout`, `semantic-ir.layout`, `lowering.layout`, `platform.layout`, `tooling.layout`. Do not duplicate IDs or replace legacy history.

## § 10. Completion boundary

P24 resolves and verifies physical **storage layout** and materializes eligible representation-sensitive operations. ABI classification/foreign wrappers are a later package; unsupported selected descriptor policies remain explicit blockers, never guessed pointers.
