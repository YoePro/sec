# P21 Normative Synchronization — Decimal, Precision and October Corrections

- **Status:** D1–D3 decided 2026-10-28 by `rules/corrections/md066-md067-md068-decimal-correction-20261028.md` (MD-066/MD-067/MD-068); the decided semantics are owned by the canonical rulebooks; implementation pending
- **Created:** 2026-10-16
- **Last updated:** 2026-10-28 (D1–D3 decided; unapproved recommendations superseded)
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Canonical repository path (proposed):** `rules/mlir/normative-sync/sec_decimal_numeric_sync_package21.md`
- **Package:** `SEC-MLIR-P21`
- **Reviewed repository:** `YoePro/sec`, `main`, October 16, 2026
- **Repository SHA:** Not independently verifiable through the accessible GitHub view; record exact SHA at implementation time.
- **Predecessor:** `SEC-MLIR-P20` (schema 16 specification)

## § 1. Sources and hierarchy

Reviewed `rules/types/types.md`, `rules/foundations/operators.md`, `rules/memory/layout.md`, `rules/library/core-library.md`, `rules/types/units.md`, `rules/errors/errorhandling.md`, `rules/compiler/compiler_known_members.md`, `rules/compiler/semantic_ir.md`, and current Sema/core files on the October `main`. This is a **proposed amendment**, not an automatic override of existing accepted decisions.

## § 2. Confirmed decimal family

`decimal`/`decimal128` are active exact decimal types. Unsuffixed fraction/exponent defaults to decimal; `m` selects family, not width; direct literal shaping is exact, no binary-float conversion. Layout components: `{i64,i32}` / `{i128,i32}`. Both retain distinct scalar identity.

## § 3. Confirmed core members

*Superseded 2026-10-28 (MD-066/MD-068).* The canonical surface is now `rules/library/core-library.md` §§ 10.1–10.7: `Scale: int32` with getter and `try set` reporting `PrecisionError`, `RoundTo(int32)`/`RoundTo(int32, RoundingMode)`/`Rescale(int32)`, properties `IsInteger`, `IsZero`, `IsPositive`, `IsNegative`, `Sign`, `Normalize()`, `ToInt*`/`ToFloat*` and three `ToString` overloads, mirrored by `decimal128`. The former `uint8` scale is rejected. The compiler registry's `Scale: int` remains an identified bug (it must become `int32`), and core's `decimal.ToString() string` stub remains inconsistent with `Result[string,StringError]`. Neither implementation is language authority.

## § 4. Confirmed rounding and errors

Only `RoundingMode` from `sec/core/rounding.sec` is canonical. An explicit `T(value, mode)` conversion rounds once after exact mathematics. Existing `PrecisionError` variants are exactly `PrecisionLoss`, `ScaleOverflow`, `PrecisionExhausted`. `ArithmeticError` is the error of checked built-in arithmetic; MD-067 (2026-10-28) appends exactly `ScaleOverflow` and `PrecisionExhausted` to its three variants for decimal operators, selected by the most-specific-applicable-error rule. It is not widened any other way, and `PrecisionError` stays the error of explicit scale and rounding operations. P20's `NumericConversionError` remains the requested checked-conversion type, subject to declaration synchronization.

## § 5. D1 — public decimal scale policy (DECIDED 2026-10-28, MD-066)

**Decision:** The public scale is a signed `int32` over its whole interval; negative scales are legal and the `0..255`/`uint8` recommendation is rejected. Each operation has a preferred scale (add/sub/div/rem: the maximum input scale; multiply: the sum), and a nonzero exact result takes the exact scale closest to it; exact zero keeps the preferred scale, clamped to `int32`. Literals keep their source scale when representable. `Scale` has a fallible exact `try set`; `Rescale` never rounds; `RoundTo` rounds once; `Normalize()` strips exact trailing zeros. Division alone rounds automatically, with nearest-even at the finest admissible scale when no exact result exists. A provably zero decimal divisor is a compile-time error.

**Owners:** `rules/types/types.md` "Decimal scale"; `rules/foundations/operators.md` "Decimal arithmetic", "Decimal division", "Decimal division and remainder by zero"; `rules/library/core-library.md` §§ 10.1, 10.5. The former recommendation in this section is superseded.

## § 6. D2 — decimal operator error type (DECIDED 2026-10-28, MD-067)

**Decision:** No `DecimalArithmeticError` is introduced; the former recommendation is rejected. Decimal operators reuse `core::ArithmeticError`, which gains exactly `ScaleOverflow` and `PrecisionExhausted`:

```sec
// Failures of checked built-in arithmetic operations.
enum ArithmeticError error {
    // Arithmetic overflow not described by a more specific applicable variant.
    Overflow

    // Division or remainder was attempted with a zero divisor.
    DivisionByZero

    // A shift count is invalid for the selected integer type.
    InvalidShift

    // No valid signed decimal scale can represent the mathematical result.
    ScaleOverflow

    // Exact decimal digits exceed the selected coefficient's capacity.
    PrecisionExhausted
}
```

Selection follows the most-specific-applicable-error rule, classified by mathematical cause rather than check order; a rounded quotient is no failure. `OverflowError`, `PrecisionError`, `RangeError`, `DivisionByZeroError` and `ConversionError` keep their method contracts and are not aliases. The schema-17 `!sec.decimal_failure_reason` may remain a compiler-internal reason domain, but `sec.decimal.error_from_reason` must construct `ArithmeticError` with these five variants.

**Owners:** `rules/library/core-library.md` § 20.1; `rules/errors/errorhandling.md` § 3.4; `rules/foundations/operators.md` "Decimal arithmetic failures"; `rules/compiler/semantic_ir.md` § 58(7).

## § 7. D3 — decimal128 public members (DECIDED 2026-10-28, MD-068)

**Decision:** `decimal128` has exactly the public members of `decimal`, with `decimal128` substituted in receiver, operand and result positions, including `Scale: int32` with `try set`. The complete inventory is `rules/library/core-library.md` § 10.4, with a comment before every declaration; `sec/core/decimal128.sec` is to be added. `Parse()` and `ToUInt*` remain deferred for both types.

## § 8. Exact literal and arithmetic intermediate

The builder must retain exact coefficient/exponent/scale source facts and use arbitrary-precision intermediate arithmetic. No Sema `int64` truncation for `decimal128`. `+`, `-`, `*`, `%` and unary minus return an exact mathematical decimal only when a representable coefficient/scale pair exists; otherwise they fail. *Superseded in part 2026-10-28 (MD-066 § 5):* `1/3` is no longer a failure. Decimal division rounds a nonterminating or over-precise quotient once with nearest-even at the finest admissible scale and returns it as an ordinary successful value (`1m / 3m == 0.3333333333333333333`, scale 19).

## § 9. October float compatibility notice

The October MD-014 correction makes **plain `float` target-sized** (f32 on a 32-bit platform, f64 on a 64-bit platform), superseding P20's hardcoded f64 statement. Fix P20 dependent float/decimal conversion plans and test target matrix; do not modify unrelated already accepted rounding policy. Track this correction separately as part of the P20 follow-up, not as a new decimal language feature.

## § 10. Safe application order

D1–D3 were merged into their owning canonical books on 2026-10-28. Merge any further approved amendment into its owning canonical book without renumbering existing section/paragraph IDs. Update compiler-known member metadata and core declarations, then the Semantic IR rulebook, dialect snapshot, lowering snapshot, and only then the implementation package/YAML status. Do not overwrite canonical schema 11 directly with v17 without satisfying and integrating intermediate package amendments. Preserve legacy YAML history and avoid duplicate governance IDs.
