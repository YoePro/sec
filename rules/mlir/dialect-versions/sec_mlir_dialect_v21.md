# P25 Sec MLIR Dialect Amendment — Planned Schema v21

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_mlir_dialect_package25.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P25`

---

## § 1. Version

This is **planned schema v21**, following P24 planned v20. Compiler-generated modules at this staged schema carry `sec.dialect_version = 21 : i32`. Do not assert the repository already implements v21.

## § 2. Two-stage call identity

High-level function/call ops keep the source semantic signature and calling-convention intent. Only after `CallABIPlan` resolution may ABI-specific ops represent part splitting, stack transport, hidden result and indirect entry/return. Retain bidirectional semantic↔ABI correspondence.

## § 3. Canonical ABI references

Prefer typed `#sec.abi.model`, `#sec.abi.signature`, `#sec.abi.classification`, `#sec.abi.call_plan`, `#sec.abi.fingerprint` or existing equivalent attrs; only immutable verified references. No new Sec source type is introduced.

## § 4. Operations

Proposed high-level ABI-transport operations:

```text
sec.abi.pack_arg
sec.abi.unpack_arg
sec.abi.prepare_indirect
sec.abi.prepare_return
sec.abi.pack_return
sec.abi.unpack_return
sec.abi.call
sec.abi.func
sec.abi.callback_bridge
sec.abi.vararg_promote
```

`sec.abi.call_plan` is a **typed metadata reference**, not a runtime op. Actual TableGen names may reuse verified existing operations.

## § 5. `sec.abi.pack_arg`

Takes a semantically prepared, ownership-verified value and emits the exact number/type of ABI parts from one classification. Carries source argument ordinal, classification and proof. Parts are transport, not independent Sec owners. Direct multi-part and Ignore are supported only per plan.

## § 6. `sec.abi.unpack_arg`

Receives verified machine ABI parts or verified indirect storage and reconstructs one semantic callee parameter under the same plan. No semantic copy occurs; preserve calling-side ownership intent and initialization facts.

## § 7. `sec.abi.prepare_indirect`

Represents compiler-managed argument temporary storage with ABI-selected role/size/alignment; not a source `RawPtr`, owned reference, implicit allocation context or source borrow. Cleanup/lifetime is transport-scoped and cannot outlive the call except when the selected ABI explicitly requires a known lifetime.

## § 8. `sec.abi.prepare_return`

Creates hidden result transport storage only when return classification requires it. Source return identity stays unchanged, result ownership transfers once. Verifier checks sret role and callee/caller initialization contract.

## § 9. `sec.abi.pack_return` / `unpack_return`

Translate one ownership-validated source result to/from the ABI part/result-storage contract. Respect P17 cleanup ordering and P24 validity. Never treat ABI coercions as Sec conversions.

## § 10. `sec.abi.call`

Represents ABI-lowered direct or indirect invocation with immutable `CallABIPlanRef`, exact ABI convention, physical parts/stack state/return transport, foreign provenance where applicable, and source call identity. Verifier checks complete match to declared callee/indirect function-pointer signature.

## § 11. `sec.abi.func`

Represents the ABI-lowered callable entry/return contract, including hidden params and source-to-part maps. Must not be offered as a source-language callable signature. Source semantic function identity is retained for diagnostics/debug.

## § 12. `sec.abi.callback_bridge`

A private C-facing thunk contract with foreign-facing C CallABIPlan, native Sec-facing CallABIPlan and validated adapter metadata. `c::callback` restrictions, attachment and no-unwind are explicit; no general export feature.

## § 13. `sec.abi.vararg_promote`

Consumes a resolved target promotion plan for already C-compatible foreign vararg; it **may not choose promotions** from source `int` or raw tokens inside the dialect. Vararg position/ABI resource state remains explicit.

## § 14. Required typed attrs

Retain `sec.abi.semantic_decl_id`, `sec.abi.source_ordinal`, `sec.abi.part_ordinal`, `sec.abi.hidden_role`, `sec.abi.convention`, `sec.abi.foreign_legality_proof`, `sec.abi.layout_ref`, `sec.abi.source_transfer_action` or canonical existing typed equivalents. No source name is canonical ABI ID.

## § 15. Verifiers

Register `--sec-verify-call-abi-plans`, `--sec-verify-abi-lowering`, `--sec-verify-ffi-boundaries`, `--sec-verify-abi-compatibility`. They check model/layout/position/parts/hidden/stack/varargs, foreign legality/unsafe provenance and immutable compatibility. Previous P24 layout/representation verifiers remain prerequisites.

## § 16. No LLVM reclassification

The schema is target-neutral physical ABI transport. No `LLVM::CallOp`/`llvm.call` is required in P25; a later lowering merely maps verified `CallABIPlan` to exact target IR encoding. Unverified ABI types or unknown selected target fail closed.
