# Sec MLIR Lowering Amendment — Version 20

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_mlir_lowering_package24.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P24`

---

## § 1. Version and stage

P24 defines planned lowering v20. Its input is verified high-level Sec MLIR schema v20 and one complete `ResolvedLayout` service under `CompilationPlan`. This is intra-procedural value/storage **representation lowering**, not function/foreign ABI lowering.

## § 2. Preconditions

Sema complete; generic specialization concrete; relevant P15–P23 verifiers passed where enabled; layout refs match plan; no unresolved default/ownership/borrow/variant failure decisions. Run v20 layout verifier before any physical projection.

## § 3. Pass inventory

Implement `--sec-resolve-layouts`, `--sec-verify-resolved-layouts`, `--sec-lower-value-representations`, `--sec-verify-representation-lowering`, `--sec-verify-layout-obligations`. Use repository pass-naming convention; do not rename existing passes just for cosmetic uniformity.

## § 4. Scalar lowering

Resolve integer target widths, plain float 32/64 by target, fixed-width float/int and decimal composite reps. Preserve semantic source kinds/validity proof until discharged. Do not convert signed/unsigned named semantics into raw signless bits prematurely.

## § 5. Struct lowering

Use exact `ResolvedFieldLayout` offsets and alignment. For value-only copy-trivial structs standard MLIR aggregate handling may be used if it preserves representation. For address-observable/nontrivial structs retain typed storage until ownership and physical placement are verified. No backend field reorder.

## § 6. Array lowering

For fixed T[N], use resolved stride; checked element displacement; no hidden length/capacity, no dereference for N=0. Preserve array construction order/partial states and P17 destruction semantics; large N is not unrolled.

## § 7. Union/Result/Option lowering

Materialize only approved selected tag/payload representation; tag and payload positions from canonical layout; guards before projections; inactive bytes not read. Distinct source variants may share physical representation without semantic conflation.

## § 8. Reference and descriptors

Materialize only when a selected plan policy and all validity/protection hooks are available. Proven ref may erase epoch metadata, dynamic epoch must have a valid runtime protection/validation path; preserve address spaces and 64-bit logical epochs unless canonical proof allows otherwise. Slice empty case must not form element pointer. Owning arrays and strings retain backing ownership/lifetime facts.

## § 9. Field/element address creation

Do not derive address from host offsets. Translate typed projection into target-correct physical address/place only after alignment/bounds/provenance are verified. Do not claim safe aligned ref to packed field with insufficient alignment.

## § 10. Memory operations

Valid typed load/store must preserve representation-validity proof, initialization/ownership transfer, address-space, alignment and volatile/MMIO effects. Basic standard MLIR MemRef/Arith/CF are allowed only when exact; do not force every Place into MemRef.

## § 11. Padding

Do not expose padding through semantic equality, hash, serialization or safe load; FFI export padding obligations remain later and must not be discharged by ordinary memory copy.

## § 12. Layout queries

Fold plan-resolved type SizeOf/AlignOf/StrideOf/FieldOffset to constants; preserve semantic distinction with string value `.SizeOf`/`.Len`. Tooling queries use identical underlying facts.

## § 13. Pass ordering and partial lowering

Resolve/verify layout → prior ownership/guard verification → eligible representation lowering → check obligation ledger → remaining Sec operations explicitly tagged/deferred until ready. Do not call a partially lowered module fully lowered; ABI/LLVM closure comes later.

## § 14. Regression

Test 32/64 target matrix, different plans, wide scalar/decimal, structs/arrays/variants, zero-sized, nontrivial ownership, dynamic reference epochs, explicit contract metadata, malformed layout caches, and schema-v19 predecessor. Exact commands/results are recorded by implementer only.

## § 15. Deferred

No C/foreign or internal call ABI, no sret/register convention, no native C string, no global allocator ABI, no implicit LLVM stage, no invented packed syntax MD-019, no resolution of P23-D1–D3 or MD-050.

## § 16. Completion

Every eligible physical projection consumes exact plan-resolved layout, every erased obligation is proven/materialized/transferred, every unsupported policy remains fail-closed with diagnostic, cache/tooling match cold canonical resolution, all installed regressions green.
