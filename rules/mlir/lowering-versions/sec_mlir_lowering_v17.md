# Sec MLIR Lowering — v17 Decimal Amendment

- **Status:** Proposed lowering v17 amendment (not yet active); its D1–D3 gates were decided 2026-10-28 (MD-066/MD-067/MD-068)
- **Created:** 2026-10-16
- **Last updated:** 2026-10-28 (division rounding, int32 scale, ArithmeticError mapping)
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Canonical repository path (proposed):** `rules/mlir/lowering-versions/sec_mlir_lowering_v17.md`
- **Package:** `SEC-MLIR-P21`
- **Reviewed repository:** `YoePro/sec`, `main`, October 16, 2026
- **Repository SHA:** Not independently verifiable through the accessible GitHub view; record exact SHA at implementation time.
- **Predecessor:** `SEC-MLIR-P20` (schema 16 specification)

## § 1. Version and sequencing

Lowering v17 requires verified schema v17 and all P20 numeric-conversion prerequisites. It does not replace the current v11 canonical implementation until the intervening packages are installed. Keep earlier pass behavior and regression fixtures intact.

## § 2. Literal materialization

Exact literal parsing and coefficient/scale checks happen before lowering. Materialize high-level decimal constants through canonical signed arbitrary-precision attributes. When plan-resolved physical representation is required, check before narrowing to signed i64/i128 and signed i32. No `f32`, `f64`, decimal context with implicit rounding, or host-long conversion.

## § 3. Precision-safe arithmetic

Use exact big-integer/rational reference semantics to define lowering. Checked target integer operations or compiler-versioned helpers may implement the behavior provided they preserve exact scale policy D1 and error classification D2, do not require a heap, and do not rely on signed integer UB. Operations that require an unresolved D1/D2 result must remain high-level or be rejected as normative-decision-blocked, not guessed.

## § 4. Division and remainder

For division, reduce exact rational and check terminating decimal representability using factors 2 and 5; never round implicitly. *Superseded in part 2026-10-28 (MD-066 § 5):* when the exact quotient has no representation, lowering implements the rounding Sema planned: one nearest-even rounding from the exact quotient at the finest admissible signed `int32` scale that keeps the whole-number part, falling back to the next coarser scale on a rounding carry. Lowering does not choose the scale independently, never double-rounds, saturates or wraps, and a rounded quotient takes the success branch. For remainder calculate truncation quotient and exact remainder; handle negative signs per `operators.md`, divisor zero via the accepted error contract. No binary `arith.divf`, `arith.remf` or nearest-remainder helper.

## § 5. Comparison

Normalize mathematical scale only inside an arbitrary-precision comparison plan, preserving source value scale. Check exact comparison regardless of trailing zeroes. Do not compare physical components lexicographically or rely on target float conversion.

## § 6. Conversions and rounding

Integer-to-decimal is exact subject to coefficient/scale representation. Decimal-to-integer requires integral/in-range unless explicit `RoundingMode` authorizes a single rounding step. Float-to-decimal converts the *exact* binary rational significand/exponent and then checks decimal representation; decimal-to-float checks exactness unless a mode is provided. `NotFinite` for NaN/infinity into decimal. Preserve one-rounding-after-unit-transform policy.

## § 7. Method lowering

Use resolved compiler-known IDs, not source method spellings. Documented decimal `.scale`, `Abs`, `Min/Max/Clamp`, classification and rounding remain high-level until representation/error policy verified. Do not implement ToString/formatting in this numerical lowering package.

## § 8. Failure branches

Run verified checked decimal guard before downstream lowering. Branch `failed=true` to error/panic, `false` to success. Result candidate never used on failure. Map specific error only by approved D2 type, reuse P10/P11 handlers; don't widen errors or change panic effects in a backend. D2 (MD-067) maps decimal reasons to `core::ArithmeticError` (`DivisionByZero`, `ScaleOverflow`, `PrecisionExhausted`, `Overflow`). A runtime zero divisor without `try` reaches the panic endpoint with reason `DivisionByZero`; a provably zero divisor never reaches lowering because Sema rejects it.

## § 9. Pass registration

```text
--sec-verify-decimal-values
--sec-verify-decimal-guards
--sec-verify-decimal-conversions
--sec-lower-decimal-core
```

Recommended order: verify source Semantic IR → canonical schema v17 verify → decimal values/conversions/guard verify → lower with proof-preserving helpers → re-verify. Do not erase evidence needed by downstream physical layout/ABI checks.

## § 10. Target capability and resource model

CompilationPlan resolves i64/i128 and i32 layout, alignment, ABI, and helper availability. A strict/bare-metal target must either support a bounded exact implementation or reject the operation with an explicit capability diagnostic. Pure decimal scalar arithmetic must not introduce hidden dynamic allocation or universal runtime dependencies.

## § 11. Verification and differential tests

Use a host-independent arbitrary-precision reference evaluator; test 64/128 boundaries, negative, zero and `int32`-boundary scales under the approved D1, exact fractions, rounded nonterminating division, large exponents, negative remainder, 32/64-bit target width interaction, round modes, and active 128/256-bit integer conversions. Compare constant folding vs helper runtime behavior and MLIR parse/print. Capture actual commands and outputs.

## § 12. No unsupported shortcuts

Forbidden: silently converting decimal operations to binary float; choosing a scale in lowering; lossy coeff truncation; treating decimal128 as not active; narrowing the `int32` scale to any other type; emitting any DecimalArithmeticError (rejected by MD-067); declaring `decimal128` methods beyond the MD-068 mirror of `decimal`; rounding any decimal operation other than division; automatically appending a hidden allocation context; calling `malloc` without an approved ABI.
