# Sec MLIR Dialect Amendment — Schema v20

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_mlir_dialect_package24.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P24`

---

## § 1. Schema version and predecessor

Planned v20 after P23 planned v19; checked-in `rules/mlir/sec_mlir_dialect.md` was at v11 when publicly reviewed. New emitter must not claim executable v20 until prerequisites and verifiers work. Compiler-generated module marker: `sec.dialect_version = 20 : i32`.

## § 2. Layout reference attribute

Add a typed plan-scoped layout-reference attribute, conceptually `#sec.layout<"L-id", "plan-key">`, or reuse existing `sec.layout_ref` typed metadata. It names one immutable canonical `ResolvedLayout`; it cannot contain a second backend-derived field list.

## § 3. Optional representation transition type

If necessary use `!sec.layout_value<T,layout-id>` as a transient internal wrapper only; prefer existing type+typed layout ref where sufficient. Never make it a new Sec source type, physical ABI or runtime token.

## § 4. Static query operations

`sec.layout.size_of`, `.align_of`, `.stride_of`, `.field_offset`: resolved compile-time results for active plan. Exact result width is the corresponding resolved Sec `uint`/target constant; verifier checks completeness and representability.

## § 5. Representation operations

`sec.repr.materialize`, `.project_field`, `.project_element`, `.project_variant`, `.load_validated`, `.store_validated`, `.check_validity`, `.discharge`. Operands/results must use existing `!sec.place`/value and approved lower storage types without losing authority, provenance, validity or ownership.

## § 6. Field and array proof requirements

Projection from struct requires exact `FieldIdentity` and recorded offset, not ordinal arithmetic. Element projection requires matching base/index/guard and recorded stride. Zero-length source cannot manufacture an element reference.

## § 7. Variant proof requirements

Variant projection requires matching active-variant guard and selected tag/payload layout. Inactive payload bytes have no semantic value.

## § 8. Reference/descriptor representation

Reference/slice/dynamic-array/string/Arena physical layout uses selected plan policy and required epoch/provenance/authority representation; verifier rejects unspecified/mismatched policy and metadata erasure.

## § 9. Typed obligation discharge

Every erased high-level check produces an explicit proof record: `StaticallyDischarged`, `RuntimeMaterialized`, `TransferredToLowerLevel`. `sec.repr.discharge` names the source obligation and exact proof/lower op.

## § 10. Verifiers

Register `--sec-verify-resolved-layouts`, `--sec-verify-representation-lowering`, `--sec-verify-layout-obligations`; run after previous ownership/place/guard verifiers and before dropping high-level metadata.

## § 11. No duplicate layout or ABI engine

Dialect operations do not choose target padding, sret, register calling classes or universal fat pointer. Missing layout facts block materialization.

## § 12. Schema acceptance

All types/attrs/ops roundtrip; required guards dominate; plan mismatch fails; bit-pattern validity is enforced; no high-level obligation disappears without proof; schema v19 predecessors remain tests.
