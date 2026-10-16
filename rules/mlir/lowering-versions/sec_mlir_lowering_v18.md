# Sec MLIR Lowering — Version 18: Named Contracts

- **Status:** Target lowering specification; gated on v17 prerequisite
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/lowering-versions/sec_mlir_lowering_v18.md`
- **Replaces:** None; additive package amendment only
- **Package:** `SEC-MLIR-P22`

---

## § 1. Preconditions

Sema completed named conversion, CTE arguments, contract order, intrinsic conversion, source/target type, ownership/Place/effect resolution. Canonical Semantic IR verifies. The dialect v17 predecessor and required P15–P17 functionality must actually be installed before enabling dependent v18 lowering. Do not invent runtime facts in this pass.

## § 2. Intrinsic-first lowering

Lower intrinsic scalar/numeric/domain conversion before any contract checker. On intrinsic failure emit the existing relevant `ConversionError` variant, not a contract error. Typed primitive conversion must remain guarded against overflow and precision loss. No unvalidated nominal result exists.

## § 3. Scalar checks

`range`: canonical typed comparison and inclusivity. `odd/even`: exact parity. `multipleOf`: safe mathematical remainder or modular algorithm with wide signed-min/-1 check where needed. `in`: source-order semantically resolved equality. `finite`: canonical float classification, decimal representation-trivial only when proven. No host-width truncation or approximate decimal-to-float substitution.

## § 4. String checks

Use resolved stringcontract plan. Logical string length = count of Unicode scalar values; value `SizeOf` = UTF-8 byte count. Both counts may appear in the same ordered check sequence. Preserve embedded NUL and non-BMP, no normalization. Do not invent the pending invalid-UTF8 semantics (MD-050).

## § 5. Collection checks

Use actual compiler-known logical element counts, not descriptor byte size or capacity. For runtime `unique`, produce deterministic direct-element equality iteration only if corresponding concrete iteration/SSA/ownership/equality is implemented and verified. Otherwise generate structured unsupported-lowering before publishing output.

## § 6. Checked failure CFG

Each predicate returns pass/fail. False creates canonical `ContractKind` and `ContractError.Violation` fields, then wraps as `ConversionError.Contract`. All failure edges use P10/P11 Result/local-handler machinery; no separate contract error dispatch. Correct first-failure identity is observable and must survive optimization.

## § 7. Nominal result

Construct target nominal value only after success of intrinsic and every required declared contract. Preserve scalar kind, unit, named ancestry, generic specialization and debug location. Named representation may be physically identical to underlying T without erasing semantic validation.

## § 8. Transactional assignment

Evaluate full RHS and all checks, then call actual P17 ownership replacement/initialization after validated result exists. Old destination destruction must occur only after success. If relevant Place/cleanup machinery is still unavailable, reject new-path feature explicitly instead of store-first lowering.

## § 9. Proof-aware elimination

Elide a predicate only with an immutable Sema proof; preserve failure priority and correct error ordinal for remaining checks. No optimization reorders predicates if externally observable error kind/index changes. Do not eliminate nominal type identity even if all checks become no-ops.

## § 10. Target coverage

Require 32-/64-bit CompilationPlan parity; wide integer bounds use arbitrary precision until safe target resolution; float semantic format follows current platform-width rule (MD-014). String Unicode semantics must not depend on locale. Noalloc/bare-metal supports scalar checks without implicit allocation.

## § 11. Passes and verifiers

```text
--sec-verify-named-contracts
--sec-lower-named-contract-predicates
```

Verify before and after lowering or pass-specific equivalent. Contract predicate must not survive to physical backend as an uninterpreted accepted value. Report capability error if target lacks required lowerable operation.

## § 12. Error compatibility

MD-012 owns `core::ConversionError`. Supersede proposed P20 `NumericConversionError` references. P21 decimal conversion uses same error; P21 arithmetic error remains an independent unresolved design. Do not emit `RangeError` or `PrecisionError` as the *conversion's* public Result error without an explicit canonical mapping to `ConversionError`.

## § 13. Independent tests

Test complete source→Sema→Semantic IR→schema v18→standard CF/Arith for covered scalar family; source-ordered failing contracts, bare-metal/noalloc, string rune and byte lengths, first-error uniqueness, round-trip errors, checked assignment transactionality once P17 available, and negative malformed-IR. Compare with legacy runtime implementations on identical valid source and supported targets.

## § 14. Deferred boundaries

No regex execution (MD-010), no fabricated UTF-8 invalid policy (MD-050), no broad collection iteration if P16/P18 not implemented, no general LLVM nominal ABI and no mandatory runtime. Report each case as a testable unsupported capability.
