# Semantic IR Amendment — Exact Decimal Values and Operations

- **Status:** Proposed additive Semantic IR v1 amendment; its D1–D3 dependencies were decided 2026-10-28 (MD-066/MD-067/MD-068) and are synchronized into `rules/compiler/semantic_ir.md` §§ 58(6)–58(7)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-28 (decided scale, division rounding and ArithmeticError mapping)
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Canonical repository path (proposed):** `rules/mlir/semantic-ir/sec_semantic_ir_decimal_package21.md`
- **Package:** `SEC-MLIR-P21`
- **Reviewed repository:** `YoePro/sec`, `main`, October 16, 2026
- **Repository SHA:** Not independently verifiable through the accessible GitHub view; record exact SHA at implementation time.
- **Predecessor:** `SEC-MLIR-P20` (schema 16 specification)

## § 1. Existing boundary

`rules/compiler/semantic_ir.md` is canonical. Decimal values/operations are resolved Sec semantics, not alternative binary floating operations. P21 is an additive Semantic IR v1 amendment, not a new Sema engine or duplicated type system.

## § 2. Value identity

`DecimalType` and `Decimal128Type` carry distinct semantic identity. Constants preserve exact coefficient, exponent/scale, source lexeme and proof. All big integers are immutable copies in the finalized IR; physical scalar component fields are not Semantic IR type identity.

## § 3. Resolved plans

Consume read-only Sema `ResolvedDecimalLiteralPlan`, `ResolvedDecimalOperatorPlan`, `ResolvedDecimalConversionPlan`, `ResolvedDecimalMemberPlan`. Operator plan carries chosen D1 scale policy and D2 error identity **only after their approval**. No Semantic IR builder may infer them from AST spelling or LLVM signedness. *Approved 2026-10-28:* the operator plan carries the concrete type and width, exact operands and mathematical result basis, preferred and selected `int32` scale, exact versus rounded-division classification, rounding mode, divisor-zero proof classification, specific failure reason, the `core::ArithmeticError` identity, provenance and `try`/panic effects (`semantic_ir.md` § 58(6)).

## § 4. Operations

```text
DecimalConstOp
DecimalUnaryOp
DecimalBinaryCheckedOp
DecimalCompareOp
DecimalScaleOp
DecimalCoreOp
DecimalRoundToCheckedOp
DecimalRescaleCheckedOp
DecimalConvertExactOp
DecimalConvertCheckedOp
DecimalErrorFromReasonOp
```

Reuse existing generic numeric conversion operations when they can precisely encode the same plan. Do not duplicate `NumericConversionError` or Result representation.

## § 5. Exact arithmetic

Mathematical intermediates are arbitrary precision. Comparison is mathematical; arithmetic is checked against accepted coefficient/scale invariants. Each operation has an explicit resolved operator kind, concrete type, source order, candidate value and validated failure reason. No hidden rounding. *Superseded in part 2026-10-28 (MD-066 § 5):* an inexact division quotient is rounded once with nearest-even at the finest admissible scale, as decided by Sema and recorded in the plan; the rounded value is a successful candidate, the failure flag stays false, and no later stage re-rounds it. `+`, `-`, `*`, `%` and unary minus remain exact or fail.

## § 6. Failure CFG

Checked result convention: `candidate, failed, reason`. Success path owns candidate; failure path receives approved exact error, which for decimal operators is `core::ArithmeticError` with the most specific of `DivisionByZero`, `ScaleOverflow`, `PrecisionExhausted` and `Overflow` (MD-067); both paths preserve source side effects and P17 cleanup. Candidate use on failure is invalid. P9/P10/P11 Result/error semantics remain unchanged.

## § 7. Conversions and modes

Typed exact conversion uses P20's NumericConversionError. A source conversion with explicit `RoundingMode` carries exact mathematical source value plus the single resolved rounding mode. Unit transforms happen before rounding. Nonfinite float-to-decimal fails NotFinite. Named contracts validate afterward.

## § 8. Core operations

Intrinsic IDs correspond to the **already declared** decimal core members; `decimal128` members require D3, which is decided (MD-068: an exact mirror of `decimal`, `core-library.md` § 10.4). `Scale` is `int32` with a transactional fallible setter. No `ToString`/formatting lowering in P21. `Abs` uses `OverflowError`; precision methods use `PrecisionError`; `Clamp` uses `RangeError`. No erroneous shared operator error substitution.

## § 9. Verifier

Reject invalid coefficient/scale, policies other than the decided D1/D2, a lost or re-rounded selected scale, a failure flag on a rounded quotient, wrong nominal error (including any `DecimalArithmeticError`), wrong source type, early rounded value, f32/f64 decimal intermediate, wrong guard dominance, candidate-on-failure, active wide type downgraded to a planned placeholder, missing literal provenance, and unproved physical narrowing.

## § 10. Deterministic printer

Print decimal kind, exact coefficient, semantic scale, literal provenance, operator/check policy, failure reason and source location deterministically. Never print only host-floating approximation or a machine composite dump.
