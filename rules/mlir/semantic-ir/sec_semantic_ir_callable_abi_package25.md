# P25 Semantic IR Amendment — Callable and Foreign ABI Intent

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_semantic_ir_callable_abi_package25.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P25`

---

## § 1. Semantic callable identity

Preserve source function type, nominal type parameters, generic concrete instantiation, source parameter ordinals/names, explicit ownership actions and Result/error channels. Physical ABI parts are not source parameters.

## § 2. Calling convention intent

Retain typed `Sec`, `C`, `system` convention intent, selected target ABIModel reference and concrete CompilationPlan identity as metadata for ABI planning. No host guess. Explicit foreign trust/safety provenance is distinct from calling convention.

## § 3. Foreign legality facts

Retain already resolved per-position foreign legality facts for `c::` scalar/aggregate types, opaque C shapes, RawPtr, call-bounded refs, C function pointer and callback, foreign varargs and promotions, source unsafe obligations. Compiler builder reads Sema facts; never reinterprets syntax.

## § 4. Layout reference

Attach P24 immutable `ResolvedLayoutRef` and semantic representation validity facts where physical transport will require them. A resolved layout does not imply a foreign-legal source type.

## § 5. Copy/move and cleanup

Every call argument and result carries its existing P17 transfer/cleanup plan. ABI marshalling can split/coerce value bytes but cannot change semantic ownership or create a new borrow.

## § 6. Compiler-hidden context

P19 `RequiresAllocationContext` is distinct from source function parameter arity. Preserve compiler-only context input/selection facts in Semantic IR; ABI whole-call planning later chooses its physical place for Sec calls only.

## § 7. Callable variety

Differentiate native direct/indirect calls, foreign imported functions, C function pointer indirect calls, callback adapter creation, native typed variadic packs, C vararg promotions; do not equate callable kinds.

## § 8. Boundary to ABI stage

Semantic IR is the verified source-semantic input, not an ABI-lowered IR. It may retain `CallABIPlanRef` after resolution as read-only annotation but the physical split/stack/sret are materialized in P25 Sec MLIR ABI stage.

## § 9. Verifier contracts

Require concrete generic source types before ABI staging, correct source transfer, legal foreign contracts, no semantic source signature mutation, no implicit native-descriptor foreign conversion, no generic export or foreign callback closure captures.
